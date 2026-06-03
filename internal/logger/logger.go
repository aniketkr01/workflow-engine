package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	traceId = "trace_id"
	spanId  = "span_id"
)

var logger *zap.Logger

func Init() error {
	var err error
	logger, err = zap.NewProduction()
	return err
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		logger.With(
			zap.String(traceId, sc.TraceID().String()),
			zap.String(spanId, sc.SpanID().String()),
		).Info(msg, fields...)
		return
	}
	logger.Info(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		logger.With(
			zap.String(traceId, sc.TraceID().String()),
			zap.String(spanId, sc.SpanID().String()),
		).Warn(msg, fields...)
		return
	}
	logger.Warn(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		logger.With(
			zap.String(traceId, sc.TraceID().String()),
			zap.String(spanId, sc.SpanID().String()),
		).Error(msg, fields...)
		return
	}
	logger.Error(msg, fields...)
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		logger.With(
			zap.String(traceId, sc.TraceID().String()),
			zap.String(spanId, sc.SpanID().String()),
		).Debug(msg, fields...)
		return
	}
	logger.Debug(msg, fields...)
}

func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		logger.With(
			zap.String(traceId, sc.TraceID().String()),
			zap.String(spanId, sc.SpanID().String()),
		).Fatal(msg, fields...)
		return
	}
	logger.Fatal(msg, fields...)
}