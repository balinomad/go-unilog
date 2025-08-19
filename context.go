package unilog

import (
	"context"
)

type ctxLoggerKey struct{}

var loggerKey = ctxLoggerKey{}

// WithLogger returns a new context with logger.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext returns logger from context.
func LoggerFromContext(ctx context.Context) Logger {
	return ctx.Value(loggerKey).(Logger)
}
