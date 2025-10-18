package api

// 负责处理所有的 API 请求

import (
"database/sql"
"log"
"net/http"

"github.com/Albert-tru/DanceMirror/service/user"
"github.com/Albert-tru/DanceMirror/service/video"
"github.com/gorilla/mux"
)

// APIServer 结构体：保存服务器需要的信息
type APIServer struct {
addr string  // 服务器地址，比如 ":8080"
db   *sql.DB // 数据库连接
}

// NewAPIServer 创建一个新的服务器实例
func NewAPIServer(addr string, db *sql.DB) *APIServer {
return &APIServer{
addr: addr,
db:   db,
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

router.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
// 检查数据库连接
if s.db != nil {
if err := s.db.Ping(); err != nil {
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

// 3. 设置静态文件服务（让浏览器可以访问上传的视频）
// 访问 /uploads/xxx.mp4 就能看到视频文件
router.PathPrefix("/uploads/").Handler(
http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

// 访问 /static/xxx.html 就能看到前端页面
router.PathPrefix("/static/").Handler(
http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

// 4. 注册用户相关的路由（注册、登录）
userStore := user.NewStore(s.db)          // 创建用户数据库操作对象
userHandler := user.NewHandler(userStore) // 创建用户处理器
userHandler.RegisterRoutes(subrouter)     // 注册路由

// 5. 注册视频相关的路由（上传、查询、删除）
videoStore := video.NewStore(s.db)
videoHandler := video.NewHandler(videoStore, userStore)
videoHandler.RegisterRoutes(subrouter)

// 6. 启动服务器，开始监听请求
log.Println("🚀 Server is running on", s.addr)
return http.ListenAndServe(s.addr, corsMiddleware(router))
}
