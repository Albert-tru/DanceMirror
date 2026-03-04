package api

import (
	"net/http"
	"strings"
)

// CORS 中间件 - 同时处理 CORS 和安全头
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置 CORS 头
		// 允许所有来源访问 API，允许常见的 HTTP 方法和头，并设置预检请求的缓存时间
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// 为静态 HTML 页面添加安全头以启用 SharedArrayBuffer
		//SharedArrayBuffer：一块可以被多个线程同时读写的共享内存区域，有了它才能支持多线程和高性能的worker
		// 只有当请求路径以 /static/ 开头且以 .html 结尾时才添加这些头
		if strings.HasPrefix(r.URL.Path, "/static/") && strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		}

		// 为上传的视频文件添加 CORP 头
		// 只有当请求路径以 /uploads/ 开头时才添加这个头
		// CORP（Cross-Origin Resource Policy）头允许浏览器在跨域请求资源时进行更严格的安全检查，防止恶意网站窃取用户数据或执行未经授权的操作。
		if strings.HasPrefix(r.URL.Path, "/uploads/") {
			w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		}

		// 处理预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 继续处理请求
		next.ServeHTTP(w, r)
	})
}
