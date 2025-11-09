package unilog

import (
	"context"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/balinomad/go-unilog/handler"
)

// logger wraps a handler.Handler to implement the Logger interface.
type logger struct {
	h          handler.Handler
	callerSkip int
}

// Ensure logger implements the Logger interface.
var (
	_ Logger        = (*logger)(nil)
	_ CallerSkipper = (*logger)(nil)
	_ Configurator  = (*logger)(nil)
)

// NewLogger creates a Logger that wraps the given handler.
func NewLogger(h handler.Handler) Logger {
	return &logger{h: h, callerSkip: 0}
}

// Log implements the Logger interface.
func (l *logger) Log(ctx context.Context, level LogLevel, msg string, keyValues ...any) {
	// Respect context cancellation
	if ctx != nil && ctx.Err() != nil {
		return
	}

	if !l.h.Enabled(level) {
		return
	}

	var pc uintptr
	if l.callerSkip >= 0 {
		var pcs [1]uintptr
		// Skip: runtime.Callers + this function + Log caller = 3 base frames
		if runtime.Callers(3+l.callerSkip, pcs[:]) > 0 {
			pc = pcs[0]
		}
	}

	record := &handler.Record{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Attrs:   convertKeyValues(keyValues),
		PC:      pc,
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

// Enabled implements the Logger interface.
func (l *logger) Enabled(level LogLevel) bool {
	return l.h.Enabled(level)
}

// With implements the Logger interface.
func (l *logger) With(keyValues ...any) Logger {
	if len(keyValues) < 2 {
		return l
	}

	chainer, ok := l.h.(handler.Chainer)
	if !ok {
		return l
	}

	newHandler := chainer.WithAttrs(convertKeyValues(keyValues))
	return &logger{h: newHandler, callerSkip: l.callerSkip}
}

// WithGroup implements the Logger interface.
func (l *logger) WithGroup(name string) Logger {
	if name == "" {
		return l
	}

	chainer, ok := l.h.(handler.Chainer)
	if !ok {
		return l
	}

	newHandler := chainer.WithGroup(name)
	return &logger{h: newHandler, callerSkip: l.callerSkip}
}

// LogWithSkip implements the CallerSkipper interface.
func (l *logger) LogWithSkip(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any) {
	if ctx != nil && ctx.Err() != nil {
		return
	}

	if !l.h.Enabled(level) {
		return
	}

	var pc uintptr
	if l.callerSkip >= 0 {
		var pcs [1]uintptr
		// Skip: runtime.Callers + LogWithSkip + caller + additional skip
		if runtime.Callers(3+l.callerSkip+skip, pcs[:]) > 0 {
			pc = pcs[0]
		}
	}

	record := &handler.Record{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Attrs:   convertKeyValues(keyValues),
		PC:      pc,
	}

	_ = l.h.Handle(ctx, record)

	switch level {
	case FatalLevel:
		os.Exit(1)
	case PanicLevel:
		panic(msg)
	}
}

// CallerSkip implements the CallerSkipper interface.
func (l *logger) CallerSkip() int {
	return l.callerSkip
}

// WithCallerSkip implements the CallerSkipper interface.
func (l *logger) WithCallerSkip(skip int) (Logger, error) {
	if skip < 0 {
		return l, ErrInvalidSourceSkip
	}
	return &logger{h: l.h, callerSkip: skip}, nil
}

// WithCallerSkipDelta implements the CallerSkipper interface.
func (l *logger) WithCallerSkipDelta(delta int) (Logger, error) {
	newSkip := l.callerSkip + delta
	if newSkip < 0 {
		return l, ErrInvalidSourceSkip
	}
	return &logger{h: l.h, callerSkip: newSkip}, nil
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

// Convenience level methods
func (l *logger) Trace(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, TraceLevel, msg, keyValues...)
}

func (l *logger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, DebugLevel, msg, keyValues...)
}

func (l *logger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, InfoLevel, msg, keyValues...)
}

func (l *logger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, WarnLevel, msg, keyValues...)
}

func (l *logger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, ErrorLevel, msg, keyValues...)
}

func (l *logger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, CriticalLevel, msg, keyValues...)
}

func (l *logger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, FatalLevel, msg, keyValues...)
}

func (l *logger) Panic(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, PanicLevel, msg, keyValues...)
}

// convertKeyValues converts alternating key-value pairs to []Attr.
func convertKeyValues(keyValues []any) []handler.Attr {
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
