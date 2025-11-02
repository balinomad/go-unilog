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

// LoggerFromContext retrieves the logger from the context.
// The boolean return value indicates if a logger was found in the context.
// If no logger is found, it returns nil and false.
func LoggerFromContext(ctx context.Context) (Logger, bool) {
	if ctx == nil {
		return nil, false
	}

	l, ok := ctx.Value(loggerKey).(Logger)
	return l, ok
}

// LoggerFromContextOrDefault retrieves the logger from the context,
// falling back to the default logger if none is present.
//
// This is a convenience function equivalent to:
//
//	logger, ok := LoggerFromContext(ctx)
//	if !ok {
//	    logger = Default()
//	}
//
// For custom fallback behavior, use LoggerFromContext directly.
func LoggerFromContextOrDefault(ctx context.Context) Logger {
	if logger, ok := LoggerFromContext(ctx); ok {
		return logger
	}

	return Default()
}
