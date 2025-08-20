package unilog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
)

// fallbackLogger provides a minimal, panic-safe Logger implementation.
// It is used as the default logger when no other logger has been configured via SetDefault.
// Safe for concurrent use by multiple goroutines.
//
// Not meant for production use.
type fallbackLogger struct {
	w   *AtomicWriter
	l   *log.Logger
	lvl atomic.Int32
}

// Ensure fallbackLogger implements the following interfaces.
var (
	_ Logger       = (*fallbackLogger)(nil)
	_ Configurator = (*fallbackLogger)(nil)
)

// newFallbackLogger creates a new fallbackLogger with the given output writer.
func newFallbackLogger(w io.Writer, level LogLevel) (*fallbackLogger, error) {
	aw, err := NewAtomicWriter(w)
	if err != nil {
		return nil, err
	}

	l := &fallbackLogger{
		w: aw,
		l: log.New(aw, "[FALLBACK] ", log.LstdFlags),
	}
	l.lvl.Store(int32(level))

	return l, nil
}

// Log prints a log message if the given level is enabled.
func (l *fallbackLogger) Log(_ context.Context, level LogLevel, msg string, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

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

	if level == LevelFatal {
		os.Exit(1)
	}
}

// Enabled returns true if the given log level is enabled.
func (l *fallbackLogger) Enabled(level LogLevel) bool {
	return level >= LogLevel(l.lvl.Load())
}

// With is a no-op for for the fallback logger. It returns itself unchanged.
func (l *fallbackLogger) With(keyValues ...any) Logger {
	return l
}

// WithGroup is a no-op for for the fallback logger. It returns itself unchanged.
func (l *fallbackLogger) WithGroup(name string) Logger {
	return l
}

func (l *fallbackLogger) SetLevel(level LogLevel) error {
	if err := ValidateLogLevel(level); err != nil {
		return err
	}
	l.lvl.Store(int32(level))
	return nil
}

// SetOutput swaps the log output atomically without blocking logging.
func (l *fallbackLogger) SetOutput(w io.Writer) error {
	return l.w.Swap(w)
}

// Debug logs a message at the debug level.
func (l *fallbackLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelDebug, msg, keyValues...)
}

// Info logs a message at the info level.
func (l *fallbackLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelInfo, msg, keyValues...)
}

// Warn logs a message at the warn level.
func (l *fallbackLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelWarn, msg, keyValues...)
}

// Error logs a message at the error level.
func (l *fallbackLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelError, msg, keyValues...)
}

// Critical logs a message at the critical level.
func (l *fallbackLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelCritical, msg, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *fallbackLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelFatal, msg, keyValues...)
}
