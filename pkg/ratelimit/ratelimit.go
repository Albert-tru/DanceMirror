package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 针对某一特定用户的计数桶
type bucket struct {
	windowStart time.Time // 当前窗口的开始时间
	count       int       // 当前窗口内的请求计数
	lastSeen    time.Time // 上次访问时间，用于清理过期的 key
}

type Limiter struct {
	mu      sync.Mutex         // 保护 buckets 的并发访问
	buckets map[string]*bucket // key: IP 地址或其他标识，value: 请求计数和窗口信息
	ops     uint64             // 请求总数，用于触发清理过期 key
}

func NewLimiter() *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
	}
}

// 限制这个 IP：在 "1 分钟" 内，只能访问 "5 次"
// limiter.Allow("192.168.1.10", 5, time.Minute)
func (l *Limiter) Allow(key string, limit int, window time.Duration) (bool, time.Time) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// 获取或创建 bucket
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{
			windowStart: now,
			count:       1,
			lastSeen:    now,
		}
		l.maybeCleanup(now)          // 每次新建 bucket 时尝试清理过期 key
		return true, now.Add(window) // 1、第一次请求，允许通过，返回窗口结束时间
	}

	// 距离上一次窗口开始，经过的时间已经超过了设定的时长
	//	之前的限制周期结束了，旧账一笔勾销
	if now.Sub(b.windowStart) >= window {
		b.windowStart = now
		b.count = 1
		b.lastSeen = now
		l.maybeCleanup(now)
		return true, now.Add(window) // 2、新窗口开始，允许通过，返回新窗口结束时间
	}

	resetAt := b.windowStart.Add(window)
	b.lastSeen = now

	// 3、请求数超过限制，拒绝请求
	if b.count >= limit {
		l.maybeCleanup(now)
		return false, resetAt
	}

	// 4、如果还在当前技术周期且次数允许，允许请求，增加计数
	b.count++
	l.maybeCleanup(now)
	return true, resetAt
}

// 定期清理过期的 key，防止内存泄漏
func (l *Limiter) maybeCleanup(now time.Time) {
	// 每 1024 次请求清理一次，删除 24h 未访问的 key
	if atomic.AddUint64(&l.ops, 1)%1024 != 0 {
		return
	}
	idleTTL := 24 * time.Hour
	for k, v := range l.buckets {
		if now.Sub(v.lastSeen) > idleTTL {
			delete(l.buckets, k)
		}
	}
}

// 获取客户端 IP 地址，优先使用 X-Forwarded-For 和 X-Real-IP 头部信息，最后回退到 RemoteAddr
func ClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
