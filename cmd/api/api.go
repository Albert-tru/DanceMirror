package api

import (
"database/sql"
"log"
"net/http"

"github.com/Albert-tru/DanceMirror/service/user"
"github.com/Albert-tru/DanceMirror/service/video"
"github.com/gorilla/mux"
)

type APIServer struct {
addr string
db   *sql.DB
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

// 2. 创建 API 路由组（所有 API 都以 /api/v1 开头）
subrouter := router.PathPrefix("/api/v1").Subrouter()

// 3. 设置静态文件服务
router.PathPrefix("/uploads/").Handler(
http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

router.PathPrefix("/static/").Handler(
http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

// 4. 根路径重定向到静态文件
router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
})

// 5. 注册用户相关的路由（注册、登录）
userStore := user.NewStore(s.db)
userHandler := user.NewHandler(userStore)
userHandler.RegisterRoutes(subrouter)

// 6. 注册视频相关的路由（上传、查询、删除）
videoStore := video.NewStore(s.db)
videoHandler := video.NewHandler(videoStore, userStore)
videoHandler.RegisterRoutes(subrouter)

// 7. 启动服务器
log.Println("🚀 Server is running on", s.addr)
log.Println("📱 访问 http://localhost:8080 开始使用")
return http.ListenAndServe(s.addr, router)
}
