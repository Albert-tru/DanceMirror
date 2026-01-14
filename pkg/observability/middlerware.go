package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

var (
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	// 注册 Metrics
	prometheus.MustRegister(HttpRequestsTotal)
	prometheus.MustRegister(HttpRequestDuration)
}

func ObservabilityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 1. 开启 Tracing Span
		tracer := otel.Tracer("http-server")
		// 从 HTTP Header 提取父 Trace (分布式追踪关键)
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path)
		defer span.End()

		// 注入 Context
		r = r.WithContext(ctx)

		// 2. 包装 ResponseWriter 捕获状态码
		rw := &responseWriter{ResponseWriter: w, status: 200}

		// 3. 执行业务逻辑
		next.ServeHTTP(rw, r)

		// 4. 记录 Metrics
		duration := time.Since(start).Seconds()
		HttpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rw.status)).Inc()
		HttpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)

		// 5. 记录 Access Log (带 Trace ID)
		Ctx(ctx).Info("http request",
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.status),
			zap.Float64("duration", duration),
		)
	})
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
