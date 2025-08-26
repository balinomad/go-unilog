package unilog

import (
	"context"
)

type ctxLoggerKey struct{}

var loggerKey = ctxLoggerKey{}

// WithLogger returns a new context with the provided logger.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext returns the logger from the context.
// The boolean return value indicates if a logger was found in the context.
// If no logger is found, it returns nil and false.
func LoggerFromContext(ctx context.Context) (Logger, bool) {
	l, ok := ctx.Value(loggerKey).(Logger)
	return l, ok
}
