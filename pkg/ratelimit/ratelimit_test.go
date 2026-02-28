package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiter_AllowWithinLimit(t *testing.T) {
	l := NewLimiter()

	key := "k1"
	limit := 3
	window := 200 * time.Millisecond

	for i := 0; i < limit; i++ {
		ok, _ := l.Allow(key, limit, window)
		if !ok {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	ok, _ := l.Allow(key, limit, window)
	if ok {
		t.Fatalf("expected request %d to be denied", limit+1)
	}
}

func TestLimiter_WindowResets(t *testing.T) {
	l := NewLimiter()

	key := "k2"
	limit := 1
	window := 50 * time.Millisecond

	ok, _ := l.Allow(key, limit, window)
	if !ok {
		t.Fatalf("expected first request to be allowed")
	}

	ok, _ = l.Allow(key, limit, window)
	if ok {
		t.Fatalf("expected second request in same window to be denied")
	}

	time.Sleep(window + 20*time.Millisecond)

	ok, _ = l.Allow(key, limit, window)
	if !ok {
		t.Fatalf("expected request after window to be allowed")
	}
}

func TestLimiter_ResetAtNotInPast(t *testing.T) {
	l := NewLimiter()

	key := "k3"
	limit := 1
	window := 80 * time.Millisecond

	_, resetAt := l.Allow(key, limit, window)
	if time.Until(resetAt) <= 0 {
		t.Fatalf("expected resetAt to be in the future, got %v", resetAt)
	}
}

func TestLimiter_ConcurrentExactLimit(t *testing.T) {
	l := NewLimiter()

	key := "k4"
	limit := 10
	window := 2 * time.Second

	var allowed int64
	var denied int64

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			ok, _ := l.Allow(key, limit, window)
			if ok {
				atomic.AddInt64(&allowed, 1)
			} else {
				atomic.AddInt64(&denied, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if allowed != int64(limit) {
		t.Fatalf("expected exactly %d allowed, got %d (denied=%d)", limit, allowed, denied)
	}
	if denied != int64(goroutines-limit) {
		t.Fatalf("expected exactly %d denied, got %d (allowed=%d)", goroutines-limit, denied, allowed)
	}
}

func BenchmarkLimiter_Allow(b *testing.B) {
	l := NewLimiter()
	key := "bench"
	limit := 100000000
	window := time.Minute

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Allow(key, limit, window)
	}
}
