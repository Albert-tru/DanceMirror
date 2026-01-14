package observability

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InitTracer 初始化全局 Tracer
func InitTracer(serviceName string) (*trace.TracerProvider, error) {
	// 从环境变量读取 OTLP endpoint（默认为本地 Jaeger）
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4318" // Jaeger OTLP HTTP endpoint
	}

	// 创建 OTLP HTTP exporter
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // 生产环境请使用 TLS
	)
	if err != nil {
		log.Printf("⚠️  Failed to create OTLP exporter, traces disabled: %v", err)
		// 返回一个 noop provider 而不是失败
		return trace.NewTracerProvider(), nil
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)
	log.Printf("✅ Tracer initialized (service=%s, endpoint=%s)", serviceName, endpoint)
	return tp, nil
}
