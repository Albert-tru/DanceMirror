package api

import (
"net/http"
"strings"
)

// CORS 中间件 - 同时处理 CORS 和安全头
func corsMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 设置 CORS 头
w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
w.Header().Set("Access-Control-Max-Age", "3600")

// 为静态 HTML 页面添加安全头以启用 SharedArrayBuffer
if strings.HasPrefix(r.URL.Path, "/static/") && strings.HasSuffix(r.URL.Path, ".html") {
w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
}

// 为上传的视频文件添加 CORP 头
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
