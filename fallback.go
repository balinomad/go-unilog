package unilog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/balinomad/go-unilog/handler"
)

// fallbackLogger provides a minimal, panic-safe Logger implementation.
// It is used as the default logger when no other logger has been configured via SetDefault,
// and as a global error recovery mechanism when handler.Handle() fails.
// Safe for concurrent use by multiple goroutines.
//
// Not meant for production use. Applications should configure a proper handler-backed logger.
type fallbackLogger struct {
	mu  sync.Mutex
	w   io.Writer
	l   *log.Logger
	lvl handler.LogLevel
}

// Ensure fallbackLogger implements Logger.
var _ Logger = (*fallbackLogger)(nil)

// newFallbackLogger creates a new fallbackLogger with the given output writer and level.
// Returns error if writer is nil or level is invalid.
func newFallbackLogger(w io.Writer, level handler.LogLevel) (*fallbackLogger, error) {
	if w == nil {
		return nil, ErrNilWriter
	}
	if err := handler.ValidateLogLevel(level); err != nil {
		return nil, err
	}

	return &fallbackLogger{
		w:   w,
		l:   log.New(w, "[FALLBACK] ", log.LstdFlags),
		lvl: level,
	}, nil
}

// newSimpleFallbackLogger creates a fallback logger with stderr output and InfoLevel.
// Never returns error; panics only if os.Stderr is nil (should never happen).
func newSimpleFallbackLogger() *fallbackLogger {
	if os.Stderr == nil {
		panic("os.Stderr is nil; cannot create fallback logger")
	}

	// Cannot fail: stderr is non-nil, InfoLevel is valid
	l, _ := newFallbackLogger(os.Stderr, handler.InfoLevel)

	return l
}

// Log prints a log message if the given level is enabled.
func (l *fallbackLogger) Log(_ context.Context, level LogLevel, msg string, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	sb := strings.Builder{}
	sb.WriteString(level.String())
	sb.WriteString(": ")
	sb.WriteString(msg)
	for i := 0; i < len(keyValues)-1; i += 2 {
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprint(keyValues[i]))
		sb.WriteString("=")
		sb.WriteString(fmt.Sprint(keyValues[i+1]))
	}

	l.l.Println(sb.String())

	// Handle termination levels
	switch level {
	case FatalLevel:
		os.Exit(1)
	case PanicLevel:
		panic(msg)
	}
}

// Enabled returns true if the given log level is enabled.
func (l *fallbackLogger) Enabled(level LogLevel) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return level >= l.lvl
}

// With is a no-op for the fallback logger. It returns itself unchanged.
func (l *fallbackLogger) With(keyValues ...any) Logger {
	return l
}

// WithGroup is a no-op for the fallback logger. It returns itself unchanged.
func (l *fallbackLogger) WithGroup(name string) Logger {
	return l
}

// Trace logs a message at the trace level.
func (l *fallbackLogger) Trace(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, TraceLevel, msg, keyValues...)
}

// Debug logs a message at the debug level.
func (l *fallbackLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, DebugLevel, msg, keyValues...)
}

// Info logs a message at the info level.
func (l *fallbackLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, InfoLevel, msg, keyValues...)
}

// Warn logs a message at the warn level.
func (l *fallbackLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, WarnLevel, msg, keyValues...)
}

// Error logs a message at the error level.
func (l *fallbackLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, ErrorLevel, msg, keyValues...)
}

// Critical logs a message at the critical level.
func (l *fallbackLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, CriticalLevel, msg, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *fallbackLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, FatalLevel, msg, keyValues...)
}

// Panic logs a message at the panic level and panics.
func (l *fallbackLogger) Panic(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, PanicLevel, msg, keyValues...)
}
