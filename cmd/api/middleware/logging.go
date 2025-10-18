package middleware

import (
	"net/http"
	"time"

	"github.com/Albert-tru/DanceMirror/utils/logger"
)

// responseWriter 包装 http.ResponseWriter 以记录状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriter 创建 responseWriter
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader 覆盖 WriteHeader 方法以记录状态码
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// RequestLogger 请求日志中间件
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// 包装 ResponseWriter 以捕获状态码
		wrapped := newResponseWriter(w)

		// 处理请求
		next.ServeHTTP(wrapped, r)

		// 计算响应时间
		duration := time.Since(startTime)

		// 记录请求日志
		logger.WithFields(map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": wrapped.statusCode,
			"duration_ms": duration.Milliseconds(),
			"ip":          r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}).Info("HTTP Request")
	})
}
