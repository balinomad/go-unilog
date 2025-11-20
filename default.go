package unilog

import (
	"context"
	"sync"
)

// packageAdditionalSkipFrame is the additional skip frames added when using
// package-level logging functions (Info, Error, etc.) compared to direct logger methods.
//
// Call stack comparison:
//
// Package function:               Direct logger method:
//  1. main.go:42 (user)            1. main.go:42 (user)
//  2. unilog.Info()                2. logger.Info()
//  3. logWithDefault()             3. logger.log()
//  4. logger.LogWithSkip()         4. [handler backend]
//  5. logger.log()
//  6. [handler backend]
//
// Package functions add 2 extra frames (unilog.Info + logWithDefault).
// These are added ON TOP OF the logger's loggerMethodSkipFrames.
const packageAdditionalSkipFrame = 2

// global is the global default logger instance.
// It is initialized on first use.
var global = struct {
	mu     sync.Mutex
	logger Logger
}{}

// globalFallback is the global fallback logger used when handler.Handle() fails.
// Initialized lazily on first error to avoid startup overhead.
var globalFallback = struct {
	once sync.Once
	l    *fallbackLogger
}{}

// getGlobalFallback returns the global fallback logger, initializing it on first call.
// Used by logger.log() when handler.Handle() returns an error.
func getGlobalFallback() *fallbackLogger {
	globalFallback.once.Do(func() {
		globalFallback.l = newSimpleFallbackLogger()
	})
	return globalFallback.l
}

// SetDefault sets the global default logger instance.
func SetDefault(l Logger) {
	global.mu.Lock()
	global.logger = l
	global.mu.Unlock()
}

// Default returns the global default logger instance. If no logger has been set,
// it initializes a fallback logger with stderr output and InfoLevel.
// Never panics; always returns a usable logger.
func Default() Logger {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.logger == nil {
		global.logger = newSimpleFallbackLogger()
	}

	return global.logger
}

// logWithDefault logs a message at the given level using the global default logger.
func logWithDefault(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any) {
	dl := Default()
	if adv, ok := dl.(AdvancedLogger); ok {
		adv.LogWithSkip(ctx, level, msg, skip+packageAdditionalSkipFrame, keyValues...)
		return
	}
	dl.Log(ctx, level, msg, keyValues...)
}

// Log logs a message at the given level using the global default logger.
func Log(ctx context.Context, level LogLevel, msg string, keyValues ...any) {
	logWithDefault(ctx, level, msg, 0, keyValues...)
}

// LogWithSkip logs a message at the given level, skipping the given number of stack frames.
func LogWithSkip(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any) {
	logWithDefault(ctx, level, msg, skip, keyValues...)
}

// Trace logs a message at the trace level using the global default logger.
func Trace(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, TraceLevel, msg, 0, keyValues...)
}

// Debug logs a message at the debug level using the global default logger.
func Debug(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, DebugLevel, msg, 0, keyValues...)
}

// Info logs a message at the info level using the global default logger.
func Info(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, InfoLevel, msg, 0, keyValues...)
}

// Warn logs a message at the warn level using the global default logger.
func Warn(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, WarnLevel, msg, 0, keyValues...)
}

// Error logs a message at the error level using the global default logger.
func Error(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, ErrorLevel, msg, 0, keyValues...)
}

// Critical logs a message at the critical level using the global default logger.
func Critical(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, CriticalLevel, msg, 0, keyValues...)
}

// Fatal logs a message at the fatal level using the global default logger and exits the process.
func Fatal(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, FatalLevel, msg, 0, keyValues...)
}

// Panic logs a message at the fatal level using the global default logger and panics.
func Panic(ctx context.Context, msg string, keyValues ...any) {
	logWithDefault(ctx, PanicLevel, msg, 0, keyValues...)
}
