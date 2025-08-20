package unilog

import (
	"context"
	"os"
	"sync"
)

var (
	defaultLogger Logger
	once          sync.Once
)

// SetDefault sets the global default logger instance.
func SetDefault(l Logger) {
	defaultLogger = l
}

// Default returns the global default logger instance. If no logger has been set,
// it initializes a fallback standard logger (stdlog) to ensure that logging
// calls do not cause a panic.
func Default() Logger {
	once.Do(func() {
		if defaultLogger == nil {
			l, err := newFallbackLogger(os.Stderr, LevelInfo)
			if err != nil {
				panic(err)
			}
			defaultLogger = l
		}
	})
	return defaultLogger
}

// Debug logs a message at the debug level using the global default logger.
func Debug(ctx context.Context, msg string, keyValues ...any) {
	Default().Debug(ctx, msg, keyValues...)
}

// Info logs a message at the info level using the global default logger.
func Info(ctx context.Context, msg string, keyValues ...any) {
	Default().Info(ctx, msg, keyValues...)
}

// Warn logs a message at the warn level using the global default logger.
func Warn(ctx context.Context, msg string, keyValues ...any) {
	Default().Warn(ctx, msg, keyValues...)
}

// Error logs a message at the error level using the global default logger.
func Error(ctx context.Context, msg string, keyValues ...any) {
	Default().Error(ctx, msg, keyValues...)
}

// Critical logs a message at the critical level using the global default logger.
func Critical(ctx context.Context, msg string, keyValues ...any) {
	Default().Critical(ctx, msg, keyValues...)
}

// Fatal logs a message at the fatal level using the global default logger and exits the process.
func Fatal(ctx context.Context, msg string, keyValues ...any) {
	Default().Fatal(ctx, msg, keyValues...)
}
