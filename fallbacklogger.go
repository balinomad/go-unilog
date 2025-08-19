package unilog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"
)

// fallbackLogger provides a minimal, panic-safe Logger implementation.
// It is used as the default logger when no other logger has been configured via SetDefault.
// Safe for concurrent use by multiple goroutines.
type fallbackLogger struct {
	w   *AtomicWriter
	l   *log.Logger
	lvl atomic.Int32
}

func (l *fallbackLogger) Sync() error { return nil }

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

	var kvs string
	if len(keyValues) > 0 {
		kvs = fmt.Sprintf(" %v", keyValues)
	}

	l.l.Printf("%s: %s%s", level.String(), msg, kvs)

	if level == LevelFatal {
		os.Exit(1)
	}
}

// With is not implemented for the fallback logger.
func (l *fallbackLogger) With(keyValues ...any) Logger { return l }

// WithGroup is not implemented for the fallback logger.
func (l *fallbackLogger) WithGroup(name string) Logger { return l }

// CallerSkip is not implemented for the fallback logger.
func (l *fallbackLogger) CallerSkip() int { return 0 }

// WithCallerSkip is not implemented for the fallback logger.
func (l *fallbackLogger) WithCallerSkip(skip int) (Logger, error) { return nil, nil }

// WithCallerSkipDelta is not implemented for the fallback logger.
func (l *fallbackLogger) WithCallerSkipDelta(delta int) (Logger, error) { return nil, nil }

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

// Enabled returns true if the given log level is enabled.
func (l *fallbackLogger) Enabled(level LogLevel) bool {
	return level >= LogLevel(l.lvl.Load())
}

// Debug logs a message at the debug level. It is a convenience wrapper around Log,
// using the current background context and the LevelDebug level.
func (l *fallbackLogger) Debug(msg string, keyValues ...any) {
	l.Log(context.Background(), LevelDebug, msg, keyValues...)
}

// Info logs a message at the info level. It is a convenience wrapper around Log,
// using the current background context and the LevelInfo level.
func (l *fallbackLogger) Info(msg string, keyValues ...any) {
	l.Log(context.Background(), LevelInfo, msg, keyValues...)
}

// Warn logs a message at the warn level. It is a convenience wrapper around Log,
// using the current background context and the LevelWarn level.
func (l *fallbackLogger) Warn(msg string, keyValues ...any) {
	l.Log(context.Background(), LevelWarn, msg, keyValues...)
}

// Error logs a message at the error level. It is a convenience wrapper around Log,
// using the current background context and the LevelError level.
func (l *fallbackLogger) Error(msg string, keyValues ...any) {
	l.Log(context.Background(), LevelError, msg, keyValues...)
}

// Critical logs a message at the critical level. It is a convenience wrapper around Log,
// using the current background context and the LevelCritical level.
func (l *fallbackLogger) Critical(msg string, keyValues ...any) {
	l.Log(context.Background(), LevelCritical, msg, keyValues...)
}

// Fatal logs a message at the fatal level. It is a convenience wrapper around Log,
// using the current background context and the LevelFatal level.
func (l *fallbackLogger) Fatal(msg string, keyValues ...any) {
	l.Log(context.Background(), LevelFatal, msg, keyValues...)
}

// DebugCtx logs a message at the debug level. It is a convenience wrapper around Log,
// using the provided context and the LevelDebug level.
func (l *fallbackLogger) DebugCtx(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelDebug, msg, keyValues...)
}

// InfoCtx logs a message at the info level. It is a convenience wrapper around Log,
// using the provided context and the LevelInfo level.
func (l *fallbackLogger) InfoCtx(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelInfo, msg, keyValues...)
}

// WarnCtx logs a message at the warn level. It is a convenience wrapper around Log,
// using the provided context and the LevelWarn level.
func (l *fallbackLogger) WarnCtx(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelWarn, msg, keyValues...)
}

// ErrorCtx logs a message at the error level. It is a convenience wrapper around Log,
// using the provided context and the LevelError level.
func (l *fallbackLogger) ErrorCtx(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelError, msg, keyValues...)
}

// CriticalCtx logs a message at the critical level. It is a convenience wrapper around Log,
// using the provided context and the LevelCritical level.
func (l *fallbackLogger) CriticalCtx(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelCritical, msg, keyValues...)
}

// FatalCtx logs a message at the fatal level. It is a convenience wrapper around Log,
// using the provided context and the LevelFatal level.
func (l *fallbackLogger) FatalCtx(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, LevelFatal, msg, keyValues...)
}
