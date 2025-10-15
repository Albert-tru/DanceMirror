package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Albert-tru/DanceMirror/service/user"
	"github.com/Albert-tru/DanceMirror/service/video"
	"github.com/gorilla/mux"
)

type APIServer struct {
	addr string
	db   *sql.DB
}

func NewAPIServer(addr string, db *sql.DB) *APIServer {
	return &APIServer{
		addr: addr,
		db:   db,
	}
}

func (s *APIServer) Run() error {
	router := mux.NewRouter()

	// 为静态文件添加 COOP/COEP 头部的辅助函数
	addCrossOriginHeaders := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
			h.ServeHTTP(w, r)
		})
	}

	// 全局中间件：Request ID、日志、体积限制等
	router.Use(s.requestIDMiddleware)
	router.Use(s.loggingMiddleware)
	router.Use(limitBodyMiddleware(500 * 1024 * 1024)) // 500MB

	subrouter := router.PathPrefix("/api/v1").Subrouter()

	// 健康检查路由（提前注册，确保优先匹配）
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	router.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// 简单检查 DB 可用性
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()
		if s.db != nil {
			if err := s.db.PingContext(ctx); err != nil {
				http.Error(w, "db not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// 静态文件服务（HTML/CSS/JS）- 包装 COOP/COEP 头部
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static")))
	router.PathPrefix("/static/").Handler(addCrossOriginHeaders(staticHandler))

	// 静态文件服务（视频文件）
	router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// 用户服务
	userStore := user.NewStore(s.db)
	userHandler := user.NewHandler(userStore)
	userHandler.RegisterRoutes(subrouter)

	// 视频服务
	videoStore := video.NewStore(s.db)
	videoHandler := video.NewHandler(videoStore, userStore)
	videoHandler.RegisterRoutes(subrouter)

	// Dump registered routes for debugging
	if err := router.Walk(func(route *mux.Route, r *mux.Router, ancestors []*mux.Route) error {
		tpl, err := route.GetPathTemplate()
		if err == nil {
			log.Println("route:", tpl)
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
		log.Println("🚀 Server is running on", s.addr)
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
			// generate simple random id
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
		// set in context for downstream use
		ctx := context.WithValue(r.Context(), "request_id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMiddleware logs basic request info with request id and duration
func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id, _ := r.Context().Value("request_id").(string)
		if id == "" {
			id = r.Header.Get("X-Request-Id")
		}
		// call next
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
