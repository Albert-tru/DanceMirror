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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Albert-tru/DanceMirror/pkg/observability"
	"github.com/Albert-tru/DanceMirror/service/admin"
	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/service/mq"
	"github.com/Albert-tru/DanceMirror/service/search"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/service/user"
	"github.com/Albert-tru/DanceMirror/service/video"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	return &APIServer{addr: addr, db: db, redisClient: redisClient, storage: storage, mq: mq, es: es}
}

func (s *APIServer) Run() error {
	router := mux.NewRouter()

	router.Use(s.requestIDMiddleware)
	router.Use(observability.ObservabilityMiddleware)
	router.Use(s.errorLogMiddleware)
	router.Use(limitBodyMiddleware(500 * 1024 * 1024))
	router.Handle("/metrics", promhttp.Handler())

	subrouter := router.PathPrefix("/api/v1").Subrouter()

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

	router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))
	fs := http.FileServer(http.Dir("./static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/index.html", http.StatusFound)
	})

	userStore := user.NewStore(s.db)
	videoStore := video.NewStore(s.db, s.mq)

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

	userHandler := user.NewHandler(userStore)
	userHandler.RegisterRoutes(subrouter)

	publishCrop := func(task map[string]interface{}) error {
		body, err := json.Marshal(task)
		if err != nil {
			return err
		}
		return s.mq.Publish("video_crop_queue", body)
	}
	publishAnalyze := func(task map[string]interface{}) error {
		body, err := json.Marshal(task)
		if err != nil {
			return err
		}
		return s.mq.Publish("video_analyze_queue", body)
	}

	videoHandler := video.NewHandler(videoStore, userStore, s.redisClient, s.storage, publishCrop, publishAnalyze, s.es)
	videoHandler.RegisterRoutes(subrouter)

	adminHandler := admin.NewHandler(s.db, userStore)
	adminHandler.RegisterRoutes(subrouter)

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("🚀 Server is running on %s", s.addr)
		log.Printf("MQ Connected: %v", s.mq != nil)
		log.Printf("ES Connected: %v", s.es != nil)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (s *APIServer) errorLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}
		if s.db == nil {
			return
		}

		requestID := utils.GetRequestIDFromContext(r.Context())
		userID := auth.GetUserIDFromContext(r.Context())
		if userID <= 0 {
			userID = auth.GetUserIDFromRequest(r)
		}
		var userIDValue interface{}
		if userID > 0 {
			userIDValue = userID
		}

		var videoIDValue interface{}
		videoID := 0
		if vars := mux.Vars(r); vars != nil {
			if rawVideoID, ok := vars["id"]; ok {
				if parsed, err := strconv.Atoi(rawVideoID); err == nil {
					videoID = parsed
					videoIDValue = parsed
				}
			}
		}
		if videoID == 0 {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			for idx := 0; idx < len(parts)-1; idx++ {
				if parts[idx] == "videos" {
					if parsed, err := strconv.Atoi(parts[idx+1]); err == nil {
						videoID = parsed
						videoIDValue = parsed
					}
					break
				}
			}
		}

		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/videos" && userID > 0 {
			status := "success"
			errorCode := ""
			if recorder.status >= 400 {
				status = "failed"
				errorCode = fmt.Sprintf("HTTP_%d", recorder.status)
			}
			_ = s.db.Exec(`
INSERT INTO upload_events (userId, videoId, status, fileSize, errorCode, requestId)
VALUES (?, NULL, ?, 0, ?, ?)
`, userID, status, errorCode, requestID).Error
		}

		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/crop") && userID > 0 && videoID > 0 {
			errorCode := ""
			if recorder.status >= 400 {
				errorCode = fmt.Sprintf("HTTP_%d", recorder.status)
			}
			_ = s.db.Exec(`
INSERT INTO upload_events (userId, videoId, status, fileSize, errorCode, requestId)
VALUES (?, ?, 'crop_requested', 0, ?, ?)
`, userID, videoID, errorCode, requestID).Error
		}

		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/analyze") && userID > 0 && videoID > 0 {
			status := "queued"
			reason := ""
			if recorder.status >= 400 {
				status = "failed"
				reason = fmt.Sprintf("HTTP_%d", recorder.status)
			}
			_ = s.db.Exec(`
INSERT INTO ai_tasks (userId, videoId, status, errorReason, requestId)
VALUES (?, ?, ?, ?, ?)
`, userID, videoID, status, reason, requestID).Error
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/analysis") && recorder.status == http.StatusOK && userID > 0 && videoID > 0 {
			_ = s.db.Exec(`
UPDATE ai_tasks
SET status='success', errorReason=NULL, durationMs=TIMESTAMPDIFF(MICROSECOND, createdAt, NOW())/1000
WHERE userId=? AND videoId=?
ORDER BY id DESC
LIMIT 1
`, userID, videoID).Error
		}

		if recorder.status < 400 {
			return
		}

		errorCode := fmt.Sprintf("HTTP_%d", recorder.status)
		message := http.StatusText(recorder.status)
		_ = s.db.Exec(`
INSERT INTO api_error_logs (requestId, userId, videoId, method, path, statusCode, errorCode, message)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, requestID, userIDValue, videoIDValue, r.Method, r.URL.Path, recorder.status, errorCode, message).Error
	})
}

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

func limitBodyMiddleware(maxBytes int64) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
