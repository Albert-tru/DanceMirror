package api

// 负责处理所有的 API 请求

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/cache"
	"github.com/Albert-tru/DanceMirror/service/mq"
	"github.com/Albert-tru/DanceMirror/service/storage"
	"github.com/Albert-tru/DanceMirror/service/user"
	"github.com/Albert-tru/DanceMirror/service/video"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// APIServer 结构体：保存服务器需要的信息
type APIServer struct {
	addr    string               // 服务器地址，比如 ":8080"
	db      *gorm.DB             // 数据库连接
	cache   *cache.RedisClient   // Redis 客户端
	storage storage.VideoStorage // 视频存储接口
}

// NewAPIServer 创建一个新的服务器实例
func NewAPIServer(addr string, db *gorm.DB, cache *cache.RedisClient, storage storage.VideoStorage) *APIServer {
	return &APIServer{
		addr:    addr,
		db:      db,
		cache:   cache,
		storage: storage,
	}
}

// Run 启动服务器
func (s *APIServer) Run() error {
	router := mux.NewRouter()

	// 健康检查
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).Methods("GET")

	// ❌ 删除：这里原本重复初始化 storage 的代码块已删除
	// 我们直接使用 s.storage，它已经在 main.go 中初始化好了

	// 静态文件服务 (仅本地存储模式)
	if config.Envs.StorageDriver == "local" {
		fileServer := http.FileServer(http.Dir(config.Envs.UploadDir))
		router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", fileServer))
	}

	// 根路径重定向
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
	}).Methods("GET")

	// API 路由组
	subrouter := router.PathPrefix("/api/v1").Subrouter()

	// 前端静态页面
	router.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// 注册用户路由
	userStore := user.NewStore(s.db)
	userHandler := user.NewHandler(userStore)
	userHandler.RegisterRoutes(subrouter)

	// 注册视频路由
	videoStore := video.NewStore(s.db)
	// ✅ 这里传入 s.storage，确保与 main.go 中一致
	videoHandler := video.NewHandler(videoStore, userStore, s.cache, s.storage, func(task map[string]interface{}) error {
		b, err := json.Marshal(task)
		if err != nil {
			return err
		}
		var ct mq.CropTask
		if err := json.Unmarshal(b, &ct); err != nil {
			return err
		}
		return mq.PublishCropTask(ct)
	})
	videoHandler.RegisterRoutes(subrouter)

	log.Println("🚀 Server is running on", s.addr)
	// ✅ 添加 CORS 中间件，防止跨域问题
	return http.ListenAndServe(s.addr, corsMiddleware(router))
}
