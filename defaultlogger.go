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

// Package-level convenience functions that use the default logger.
func Debug(msg string, keyValues ...any)    { Default().Debug(msg, keyValues...) }
func Info(msg string, keyValues ...any)     { Default().Info(msg, keyValues...) }
func Warn(msg string, keyValues ...any)     { Default().Warn(msg, keyValues...) }
func Error(msg string, keyValues ...any)    { Default().Error(msg, keyValues...) }
func Critical(msg string, keyValues ...any) { Default().Critical(msg, keyValues...) }
func Fatal(msg string, keyValues ...any)    { Default().Fatal(msg, keyValues...) }

func DebugCtx(ctx context.Context, msg string, keyValues ...any) {
	Default().DebugCtx(ctx, msg, keyValues...)
}
func InfoCtx(ctx context.Context, msg string, keyValues ...any) {
	Default().InfoCtx(ctx, msg, keyValues...)
}
func WarnCtx(ctx context.Context, msg string, keyValues ...any) {
	Default().WarnCtx(ctx, msg, keyValues...)
}
func ErrorCtx(ctx context.Context, msg string, keyValues ...any) {
	Default().ErrorCtx(ctx, msg, keyValues...)
}
func CriticalCtx(ctx context.Context, msg string, keyValues ...any) {
	Default().CriticalCtx(ctx, msg, keyValues...)
}
func FatalCtx(ctx context.Context, msg string, keyValues ...any) {
	Default().FatalCtx(ctx, msg, keyValues...)
}
