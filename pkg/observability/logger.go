package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var Log *zap.Logger

// InitLogger 初始化全局结构化日志
func InitLogger() {
	var err error
	// 生产环境用 NewProduction (JSON格式)
	Log, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

// Ctx 从 Context 中提取 TraceID 并返回带 trace_id 的 logger
func Ctx(ctx context.Context) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return Log
	}

	// 自动在日志中注入 trace_id, span_id
	return Log.With(
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)
}
