package middleware

import (
	"net/http"
	"sync"

	"github.com/Albert-tru/DanceMirror/utils"
	"golang.org/x/time/rate"
)

// IPRateLimiter IP 限流器
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

// NewIPRateLimiter 创建 IP 限流器
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}
}

// GetLimiter 获取 IP 对应的限流器
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// 全局限流器：每秒10个请求，最多20个突发请求
var GlobalRateLimiter = NewIPRateLimiter(10, 20)

// 严格限流器：每秒3个请求，最多5个突发请求（用于登录等敏感接口）
var StrictRateLimiter = NewIPRateLimiter(3, 5)

// 上传限流器：每秒1个请求，最多2个突发请求
var UploadRateLimiter = NewIPRateLimiter(1, 2)

// RateLimit 限流中间件
func RateLimit(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 获取客户端 IP
			ip := r.RemoteAddr

			// 尝试从 X-Forwarded-For 或 X-Real-IP 获取真实 IP
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
				ip = realIP
			}

			// 获取该 IP 的限流器并检查
			rateLimiter := limiter.GetLimiter(ip)
			if !rateLimiter.Allow() {
				utils.TooManyRequests(w, "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
