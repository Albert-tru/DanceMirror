package api

// 负责处理所有的 API 请求

import (
	"log"
	"net/http"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/service/cache"
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
func NewAPIServer(addr string, db *gorm.DB, cache *cache.RedisClient) *APIServer {
	return &APIServer{
		addr:  addr,
		db:    db,
		cache: cache,
	}
}

// Run 启动服务器的主函数
func (s *APIServer) Run() error {
	// 1. 创建路由器（负责管理所有的 URL 路径）
	router := mux.NewRouter()

	// 健康检查路由（用于监控和负载均衡器探测）
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).Methods("GET")

	//初始化存储层
	var storageClient storage.VideoStorage
	var err error

	if config.Envs.StorageDriver == "minio" {
		storageClient, err = storage.NewMinIOStorage(
			config.Envs.MinIOEndpoint,
			config.Envs.MinIOAccessKey,
			config.Envs.MinIOSecretKey,
			config.Envs.MinIOBucket,
			config.Envs.MinIOUseSSL,
		)
		if err != nil {
			log.Fatalf("❌ MinIO初始化失败: %v", err)
		}
		log.Println("✅ MinIO存储初始化成功")
	} else {
		storageClient = storage.NewLocalStorage("./uploads")
		log.Println("✅ 本地存储初始化成功")
	}

	// ⭐ 关键步骤：如果是本地存储，必须开启静态文件服务
	if config.Envs.StorageDriver == "local" {
		// 将 URL 路径 /uploads/ 映射到本地目录 ./uploads
		fileServer := http.FileServer(http.Dir(config.Envs.UploadDir))
		router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", fileServer))
	}

	s.storage = storageClient

	router.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// 检查数据库连接
		if s.db != nil {
			sqlDB, err := s.db.DB()
			if err != nil {
				http.Error(w, "database not ready", http.StatusServiceUnavailable)
				return
			}
			if err := sqlDB.Ping(); err != nil {
				http.Error(w, "database not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	}).Methods("GET")

	// 根路径重定向到 /static/
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
	}).Methods("GET")

	// 2. 创建 API 路由组（所有 API 都以 /api/v1 开头）
	subrouter := router.PathPrefix("/api/v1").Subrouter()

	// ⭐ 只在本地存储时才启用静态文件服务
	if config.Envs.StorageDriver != "minio" {
		router.PathPrefix("/uploads/").Handler(
			http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.Envs.UploadDir))))
	}

	// 访问 /static/xxx.html 就能看到前端页面
	router.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// 4. 注册用户相关的路由（注册、登录）
	userStore := user.NewStore(s.db)          // 创建用户数据库操作对象
	userHandler := user.NewHandler(userStore) // 创建用户处理器
	userHandler.RegisterRoutes(subrouter)     // 注册路由

	// 5. 注册视频相关的路由（上传、查询、删除）
	videoStore := video.NewStore(s.db)
	videoHandler := video.NewHandler(videoStore, userStore, s.cache, s.storage)
	videoHandler.RegisterRoutes(subrouter)

	// 6. 启动服务器，开始监听请求
	log.Println("🚀 Server is running on", s.addr)
	return http.ListenAndServe(s.addr, corsMiddleware(router))
}
