package unilog

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/balinomad/go-unilog/handler"
)

// logger wraps a handler.Handler to implement the Logger interface.
type logger struct {
	h    handler.Handler
	skip int
}

// Ensure logger implements the Logger interface.
var (
	_ Logger         = (*logger)(nil)
	_ AdvancedLogger = (*logger)(nil)
	_ Configurator   = (*logger)(nil)
)

// internalSkipFrames is the number of stack frames this handler adds
// between unilog.Logger.Log() and the backend logger call.
//
// Frames to skip:
//
//	1: logger.Log, logger.LogWithSkip, or convenience methods (e.g. logger.Info, logger.Error)
//	2: logger.log
const internalSkipFrames = 2

// NewLogger creates a new logger that wraps the given handler.
func NewLogger(h handler.Handler) (Logger, error) {
	return NewAdvancedLogger(h)
}

// NewAdvancedLogger creates a new advanced logger that wraps the given handler.
func NewAdvancedLogger(h handler.Handler) (AdvancedLogger, error) {
	if h == nil {
		return nil, errors.New("handler cannot be nil")
	}
	return &logger{h: h}, nil
}

// log logs a message at the given level.
func (l *logger) log(ctx context.Context, level LogLevel, msg string, skipDelta int, keyValues ...any) {
	// Respect context cancellation
	if ctx != nil && ctx.Err() != nil {
		return
	}

	if !l.h.Enabled(level) {
		return
	}

	// Skip: runtime.Callers + LogWithSkip
	skip := max(l.skip+skipDelta, 0)
	var pc uintptr
	var pcs [1]uintptr
	if runtime.Callers(internalSkipFrames+skip, pcs[:]) > 0 {
		pc = pcs[0]
	}

	record := &handler.Record{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Attrs:   keyValuePairsToAttrs(keyValues),
		PC:      pc,
		Skip:    skip,
	}

	// Ignore handler errors (logging must not crash the application)
	_ = l.h.Handle(ctx, record)

	// Handle termination levels
	switch level {
	case FatalLevel:
		os.Exit(1)
	case PanicLevel:
		panic(msg)
	}
}

// Log is the generic logging entry point. It implements the Logger interface.
// Logging on Fatal and Panic levels will exit the process.
func (l *logger) Log(ctx context.Context, level LogLevel, msg string, keyValues ...any) {
	l.log(ctx, level, msg, 0, keyValues...)
}

// Enabled reports whether logging at the given level is currently enabled.
func (l *logger) Enabled(level LogLevel) bool {
	return l.h.Enabled(level)
}

// With returns a new Logger that always includes the given key-value pairs,
// if the underlying handler supports it.
// Implementations should treat this immutably (original logger unchanged).
func (l *logger) With(keyValues ...any) Logger {
	if len(keyValues) < 2 {
		return l
	}
	chainer, ok := l.h.(handler.Chainer)
	if !ok {
		return l
	}
	return &logger{
		h:    chainer.WithAttrs(keyValuePairsToAttrs(keyValues)),
		skip: l.skip,
	}
}

// WithGroup returns a new Logger that starts a key-value group,
// if the underlying handler supports it.
// If name is non-empty, keys of attributes will be qualified with it.
func (l *logger) WithGroup(name string) Logger {
	if name == "" {
		return l
	}
	chainer, ok := l.h.(handler.Chainer)
	if !ok {
		return l
	}
	return &logger{
		h:    chainer.WithGroup(name),
		skip: l.skip,
	}
}

// LogWithSkip implements the CallerSkipper interface.
func (l *logger) LogWithSkip(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any) {
	l.log(ctx, level, msg, l.skip-skip, keyValues...)
}

// WithCallerSkip implements the CallerSkipper interface.
func (l *logger) WithCallerSkip(skip int) AdvancedLogger {
	return &logger{
		h:    l.h,
		skip: max(skip, 0),
	}
}

// WithCallerSkipDelta implements the CallerSkipper interface.
func (l *logger) WithCallerSkipDelta(delta int) AdvancedLogger {
	return &logger{
		h:    l.h,
		skip: max(l.skip+delta, 0),
	}
}

// SetLevel implements the Configurator interface if the handler supports it.
func (l *logger) SetLevel(level LogLevel) error {
	if cfg, ok := l.h.(Configurator); ok {
		return cfg.SetLevel(level)
	}
	return nil
}

// SetOutput implements the Configurator interface if the handler supports it.
func (l *logger) SetOutput(w io.Writer) error {
	if cfg, ok := l.h.(Configurator); ok {
		return cfg.SetOutput(w)
	}
	return nil
}

// Sync implements the Syncer interface if the handler supports it.
func (l *logger) Sync() error {
	if syncer, ok := l.h.(Syncer); ok {
		return syncer.Sync()
	}
	return nil
}

// Trace is a convenience method that logs a message at the trace level.
func (l *logger) Trace(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, TraceLevel, msg, 0, keyValues...)
}

// Debug is a convenience method that logs a message at the debug level.
func (l *logger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, DebugLevel, msg, 0, keyValues...)
}

// Info is a convenience method that logs a message at the info level.
func (l *logger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, InfoLevel, msg, 0, keyValues...)
}

// Warn is a convenience method that logs a message at the warn level.
func (l *logger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, WarnLevel, msg, 0, keyValues...)
}

// Error is a convenience method that logs a message at the error level.
func (l *logger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, ErrorLevel, msg, 0, keyValues...)
}

// Critical is a convenience method that logs a message at the critical level.
func (l *logger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, CriticalLevel, msg, 0, keyValues...)
}

// Fatal is a convenience method that logs a message at the fatal level and then exits.
func (l *logger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, FatalLevel, msg, 0, keyValues...)
}

// Panic is a convenience method that logs a message at the panic level and then panics.
func (l *logger) Panic(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, PanicLevel, msg, 0, keyValues...)
}

// convertKeyValues converts alternating key-value pairs to []Attr.
func keyValuePairsToAttrs(keyValues []any) []handler.Attr {
	if len(keyValues) == 0 {
		return nil
	}

	attrs := make([]handler.Attr, 0, len(keyValues)/2)
	for i := 0; i < len(keyValues)-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, handler.Attr{Key: key, Value: keyValues[i+1]})
	}
	return attrs
}
