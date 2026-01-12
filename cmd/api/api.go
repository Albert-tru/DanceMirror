package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/service/mq"
	"github.com/Albert-tru/DanceMirror/service/search"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/service/user"
	"github.com/Albert-tru/DanceMirror/service/video"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type APIServer struct {
	addr        string
	db          *gorm.DB
	redisClient *cache.RedisClient
	storage     storage.VideoStorage
	mq          *mq.RabbitMQClient
	es          *search.ESClient
}

func NewAPIServer(
	addr string,
	db *gorm.DB,
	redisClient *cache.RedisClient,
	storage storage.VideoStorage,
	mq *mq.RabbitMQClient,
	es *search.ESClient,
) *APIServer {
	return &APIServer{
		addr:        addr,
		db:          db,
		redisClient: redisClient,
		storage:     storage,
		mq:          mq,
		es:          es,
	}
}

func (s *APIServer) Run() error {
	router := mux.NewRouter()

	// 全局中间件：Request ID、日志、体积限制等
	router.Use(s.requestIDMiddleware)
	router.Use(s.loggingMiddleware)
	router.Use(limitBodyMiddleware(500 * 1024 * 1024)) // 500MB

	subrouter := router.PathPrefix("/api/v1").Subrouter()

	// 健康检查路由
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	router.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()
		if s.db != nil {
			db, _ := s.db.DB()
			if err := db.PingContext(ctx); err != nil {
				http.Error(w, "db not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// uploads
	router.PathPrefix("/uploads/").Handler(
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// static (frontend)
	fs := http.FileServer(http.Dir("./static"))

	// ✅ 修复：直接使用 FileServer，去除手动修改 Path 的闭包
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	// ✅ 新增：访问根目录 http://localhost:8080/ 时自动跳到首页
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/index.html", http.StatusFound)
	})

	// 初始化 Services
	userStore := user.NewStore(s.db)
	videoStore := video.NewStore(s.db, s.mq)

	// ES 初始化（若未传入则尝试从环境创建）
	if s.es == nil {
		esAddr := os.Getenv("ES_ADDR")
		if esAddr == "" {
			esAddr = os.Getenv("ELASTICSEARCH_URL")
		}
		if esAddr != "" {
			if esClient, err := search.NewESClient(esAddr); err != nil {
				log.Printf("ES init failed: %v (addr=%s)", err, esAddr)
			} else {
				s.es = esClient
			}
		} else {
			log.Printf("ES not configured; search features disabled")
		}
	}

	// User Handler
	userHandler := user.NewHandler(userStore)
	userHandler.RegisterRoutes(subrouter)

	// Video Handler with Dependencies
	publishCrop := func(task map[string]interface{}) error {
		body, err := json.Marshal(task)
		if err != nil {
			return err
		}
		// 使用 main.go 中定义的队列名 "video_crop_queue"
		return s.mq.Publish("video_crop_queue", body)
	}

	videoHandler := video.NewHandler(videoStore, userStore, s.redisClient, s.storage, publishCrop, s.es)
	videoHandler.RegisterRoutes(subrouter)

	// Debug Routes
	if err := router.Walk(func(route *mux.Route, r *mux.Router, ancestors []*mux.Route) error {
		_, err := route.GetPathTemplate()
		if err == nil {
			// log.Println("route:", tpl)
		}
		return nil
	}); err != nil {
		log.Println("route walk err:", err)
	}

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 启动服务器
	go func() {
		log.Printf("🚀 Server is running on %s", s.addr)
		// 打印 Redis/MQ 状态 (可选)
		log.Printf("MQ Connected: %v", s.mq != nil)
		log.Printf("ES Connected: %v", s.es != nil)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exiting")
	return nil
}

// requestIDMiddleware injects X-Request-Id if absent
func (s *APIServer) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			} else {
				id = fmt.Sprintf("rid-%d", time.Now().UnixNano())
			}
			w.Header().Set("X-Request-Id", id)
		} else {
			w.Header().Set("X-Request-Id", id)
		}
		ctx := context.WithValue(r.Context(), "request_id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMiddleware logs basic request info
func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id, _ := r.Context().Value("request_id").(string)
		if id == "" {
			id = r.Header.Get("X-Request-Id")
		}
		next.ServeHTTP(w, r)
		dur := time.Since(start)
		log.Printf("req=%s method=%s path=%s remote=%s dur=%s", id, r.Method, r.URL.Path, r.RemoteAddr, dur)
	})
}

// limitBodyMiddleware limits request body size
func limitBodyMiddleware(maxBytes int64) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
