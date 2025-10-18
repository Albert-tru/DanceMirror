package api

import (
"net/http"
)

// CORS 中间件
func corsMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 设置 CORS 头
w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
w.Header().Set("Access-Control-Max-Age", "3600")

// 处理预检请求
if r.Method == "OPTIONS" {
w.WriteHeader(http.StatusOK)
return
}

// 继续处理请求
next.ServeHTTP(w, r)
})
}
