package unilog

import (
	"context"
	"os"
	"sync"
)

// global is the global default logger instance.
// It is initialized on first use.
var global = struct {
	mu     sync.Mutex
	logger Logger
}{}

// SetDefault sets the global default logger instance.
func SetDefault(l Logger) {
	global.mu.Lock()
	global.logger = l
	global.mu.Unlock()
}

// Default returns the global default logger instance. If no logger has been set,
// it initializes a fallback standard logger (stdlog) to ensure that logging
// calls do not cause a panic.
func Default() Logger {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.logger == nil {
		l, err := newFallbackLogger(os.Stderr, InfoLevel)
		if err != nil {
			panic(err)
		}
		global.logger = l
	}

	return global.logger
}

// logWithDefault logs a message at the given level using the global default logger.
func logWithDefault(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any) {
	dl := Default()
	if skipper, ok := dl.(CallerSkipper); ok {
		// Skip two additional frames to account for this function and the caller
		skipper.LogWithSkip(ctx, level, msg, skip+2, keyValues...)
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
