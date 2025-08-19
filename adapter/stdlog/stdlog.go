package stdlog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"

	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-unilog"
)

// DefaultKeySeparator is the default separator for group key prefixes.
const DefaultKeySeparator = "_"

// internalSkipFrames is the number of frames to skip within this adapter.
const internalSkipFrames = 2

// fieldStringer returns a string representation of a key-value pair.
var fieldStringer = func(k string, v any) string { return k + "=" + fmt.Sprint(v) }

// stdLogger is a unilog.Logger implementation for Go's standard library log package.
type stdLogger struct {
	l          *log.Logger
	out        *unilog.AtomicWriter
	lvl        atomic.Int32
	fields     *unilog.KeyValueMap
	withCaller bool
	withTrace  bool
	skipCaller atomic.Int32
}

// Ensure stdLogger implements unilog.Logger.
var _ unilog.Logger = (*stdLogger)(nil)

// New creates a new unilog.Logger instance backed by the standard log.
func New(opts ...LogOption) (unilog.Logger, error) {
	o := &logOptions{
		level:     unilog.LevelInfo,
		output:    os.Stderr,
		separator: DefaultKeySeparator,
	}

	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, unilog.ErrFailedOption(err)
		}
	}

	aw, err := unilog.NewAtomicWriter(o.output)
	if err != nil {
		return nil, unilog.ErrAtomicWriterFail(err)
	}

	l := &stdLogger{
		l:          log.New(aw, "", log.LstdFlags),
		out:        aw,
		fields:     unilog.NewKeyValueMap(o.separator, " ", fieldStringer),
		withCaller: o.withCaller,
		withTrace:  o.withTrace,
	}
	l.lvl.Store(int32(o.level))
	l.skipCaller.Store(int32(o.skipCaller + internalSkipFrames))

	return l, nil
}

// Sync is a no-op because the standard log does not buffer log output.
func (l *stdLogger) Sync() error {
	return nil
}

// log is the internal logging function used by the unilog.Logger interface. It adds caller and
// stack trace information before passing the record to the underlying slog logger.
func (l *stdLogger) log(level unilog.LogLevel, msg string, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	fields := l.fields.WithPairs(keyValues...)

	if l.withCaller {
		skip := int(l.skipCaller.Load())
		fields.Set("source", caller.New(skip).Location())
	}

	if l.withTrace && level >= unilog.LevelError {
		fields.Set("stack", string(debug.Stack()))
	}

	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(level.String())
	sb.WriteString("] ")
	sb.WriteString(msg)
	if fields.Len() > 0 {
		sb.WriteString(" ")
		sb.WriteString(fields.String())
	}

	l.l.Println(sb.String())

	if level == unilog.LevelFatal {
		os.Exit(1)
	}
}

// Log implements the unilog.Logger interface for the standard logger.
func (l *stdLogger) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(level, msg, keyValues...)
}

// With returns a new logger with the provided keyValues added to the context.
func (l *stdLogger) With(keyValues ...any) unilog.Logger {
	if len(keyValues) < 2 {
		return l
	}

	// Return a new logger with the combined fields
	clone := &stdLogger{
		l:          l.l,
		out:        l.out,
		fields:     l.fields.WithPairs(keyValues),
		withCaller: l.withCaller,
		withTrace:  l.withTrace,
	}
	clone.lvl.Store(l.lvl.Load())
	clone.skipCaller.Store(l.skipCaller.Load())

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (l *stdLogger) WithGroup(name string) unilog.Logger {
	if name == "" {
		return l
	}

	// Return a new logger with the new group prefix
	clone := &stdLogger{
		l:          l.l,
		out:        l.out,
		fields:     l.fields.WithPrefix(name),
		withCaller: l.withCaller,
		withTrace:  l.withTrace,
	}
	clone.lvl.Store(l.lvl.Load())
	clone.skipCaller.Store(l.skipCaller.Load())

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (l *stdLogger) SetLevel(level unilog.LogLevel) error {
	if err := unilog.ValidateLogLevel(level); err != nil {
		return err
	}
	l.lvl.Store(int32(level))
	return nil
}

// SetOutput sets the log destination.
func (l *stdLogger) SetOutput(w io.Writer) error {
	return l.out.Swap(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (l *stdLogger) CallerSkip() int {
	return int(l.skipCaller.Load() - internalSkipFrames)
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
// The original logger is not modified.
func (l *stdLogger) WithCallerSkip(skip int) (unilog.Logger, error) {
	if skip < 0 {
		return l, unilog.ErrInvalidSourceSkip
	}
	if skip == l.CallerSkip() {
		return l, nil
	}

	clone := &stdLogger{
		l:          l.l,
		out:        l.out,
		withTrace:  l.withTrace,
		withCaller: l.withCaller,
	}
	clone.lvl.Store(l.lvl.Load())
	clone.skipCaller.Store(int32(skip + internalSkipFrames))

	return clone, nil
}

// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
// The original logger is not modified.
func (l *stdLogger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}
	return l.WithCallerSkip(l.CallerSkip() + delta)
}

// Enabled checks if the given log level is enabled.
func (l *stdLogger) Enabled(level unilog.LogLevel) bool {
	return level >= unilog.LogLevel(l.lvl.Load())
}

// Debug logs a message at the debug level. It is a convenience wrapper around Log,
// using the current background context and the LevelDebug level.
func (l *stdLogger) Debug(msg string, keyValues ...any) {
	l.log(unilog.LevelDebug, msg, keyValues...)
}

// Info logs a message at the info level. It is a convenience wrapper around Log,
// using the current background context and the LevelInfo level.
func (l *stdLogger) Info(msg string, keyValues ...any) {
	l.log(unilog.LevelInfo, msg, keyValues...)
}

// Warn logs a message at the warn level. It is a convenience wrapper around Log,
// using the current background context and the LevelWarn level.
func (l *stdLogger) Warn(msg string, keyValues ...any) {
	l.log(unilog.LevelWarn, msg, keyValues...)
}

// Error logs a message at the error level. It is a convenience wrapper around Log,
// using the current background context and the LevelError level.
func (l *stdLogger) Error(msg string, keyValues ...any) {
	l.log(unilog.LevelError, msg, keyValues...)
}

// Critical logs a message at the critical level. It is a convenience wrapper around Log,
// using the current background context and the LevelCritical level.
func (l *stdLogger) Critical(msg string, keyValues ...any) {
	l.log(unilog.LevelCritical, msg, keyValues...)
}

// Fatal logs a message at the fatal level. It is a convenience wrapper around Log,
// using the current background context and the LevelFatal level.
func (l *stdLogger) Fatal(msg string, keyValues ...any) {
	l.log(unilog.LevelFatal, msg, keyValues...)
}

// DebugCtx logs a message at the debug level. It is a convenience wrapper around Log,
// using the provided context and the LevelDebug level.
func (l *stdLogger) DebugCtx(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelDebug, msg, keyValues...)
}

// InfoCtx logs a message at the info level. It is a convenience wrapper around Log,
// using the provided context and the LevelInfo level.
func (l *stdLogger) InfoCtx(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelInfo, msg, keyValues...)
}

// WarnCtx logs a message at the warn level. It is a convenience wrapper around Log,
// using the provided context and the LevelWarn level.
func (l *stdLogger) WarnCtx(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelWarn, msg, keyValues...)
}

// ErrorCtx logs a message at the error level. It is a convenience wrapper around Log,
// using the provided context and the LevelError level.
func (l *stdLogger) ErrorCtx(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelError, msg, keyValues...)
}

// CriticalCtx logs a message at the critical level. It is a convenience wrapper around Log,
// using the provided context and the LevelCritical level.
func (l *stdLogger) CriticalCtx(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelCritical, msg, keyValues...)
}

// FatalCtx logs a message at the fatal level. It is a convenience wrapper around Log,
// using the provided context and the LevelFatal level.
func (l *stdLogger) FatalCtx(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelFatal, msg, keyValues...)
}
