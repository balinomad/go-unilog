package unilog

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/balinomad/go-unilog/handler"
)

// handlerEntry caches the core components of a logger handler.
type handlerEntry struct {
	h         handler.Handler
	ch        handler.Chainer
	adv       handler.AdvancedHandler
	cf        handler.Configurator
	snc       handler.Syncer
	state     handler.HandlerState
	needsPC   bool // true if the record must have a program counter
	needsSkip bool // true if the record must have a caller skip
	skip      int  // current caller skip in handler
}

// newHandlerEntry wraps an already configured handler.Handler
// with additional metadata and helper functions (interface caching and flag setting).
// The caller is responsible for ensuring the handler is configured with the correct skip/level.
func newHandlerEntry(h handler.Handler) handlerEntry {
	if h == nil {
		panic("handler cannot be nil")
	}

	state := h.HandlerState()

	entry := handlerEntry{
		h:     h,
		state: state,
		// skip will be set by the caller
		needsPC:   true,
		needsSkip: false,
	}

	// Optional interfaces
	if adv, ok := h.(handler.AdvancedHandler); ok {
		entry.adv = adv
		entry.needsPC = false
		entry.needsSkip = state != nil && state.CallerEnabled()
	}
	if ch, ok := h.(handler.Chainer); ok {
		entry.ch = ch
	}
	if cf, ok := h.(handler.Configurator); ok {
		entry.cf = cf
	}
	if snc, ok := h.(handler.Syncer); ok {
		entry.snc = snc
	}

	return entry
}

// newHandlerEntryWithSkip wraps a handler.Handler with additional metadata and helper functions.
// It is used internally by the logger implementation to optimize the logging process.
// If skip is valid, this function may produce a new handler with a different caller skip.
// If skip is not valid, or the handler does not support caller skip, the original handler is returned.
// Nil handler.Handler is not allowed.
func newHandlerEntryWithSkip(h handler.Handler, skip int) handlerEntry {
	if h == nil {
		panic("handler cannot be nil")
	}

	// Set caller skip in the handler
	skip = max(skip, 0)
	if ah, ok := h.(handler.AdvancedHandler); ok && skip > 0 {
		h = ah.WithCallerSkip(skip)
	}

	entry := newHandlerEntry(h)
	entry.skip = skip

	return entry
}

// logger wraps a handler.Handler to implement the Logger interface.
// It is thread-safe.
type logger struct {
	handlerEntry              // cached handler components
	skip         int          // current caller skip
	mu           sync.RWMutex // mutex for thread-safe operations
}

// Ensure logger implements the AdvancedLogger interface, which extends Logger.
var _ AdvancedLogger = (*logger)(nil)

// internalSkipFrames is the number of stack frames this handler adds
// between unilog.Logger.Log() and the backend logger call.
// It will be ignored if the backend logger does not support caller-skip natively.
//
// Frames to skip:
//
//  1. logger.Log(), logger.LogWithSkip(), or convenience methods (e.g. logger.Info(), logger.Error())
//  2. logger.log()
//  3. handler.Handle()
const internalSkipFrames = 3

// NewLogger creates a new logger that wraps the given handler.
func NewLogger(h handler.Handler) (Logger, error) {
	return NewAdvancedLogger(h)
}

// NewAdvancedLogger creates a new advanced logger that wraps the given handler.
func NewAdvancedLogger(h handler.Handler) (AdvancedLogger, error) {
	if h == nil {
		return nil, errors.New("handler cannot be nil")
	}

	entry := newHandlerEntryWithSkip(h, internalSkipFrames)

	return &logger{
		handlerEntry: entry,
		skip:         entry.skip,
	}, nil
}

// Ensure logger implements the Logger interface.
var (
	_ Logger         = (*logger)(nil)
	_ AdvancedLogger = (*logger)(nil)
	_ MutableLogger  = (*logger)(nil)
)

// log logs a message at the given level.
func (l *logger) log(ctx context.Context, level LogLevel, msg string, delta int, keyValues ...any) {
	// Respect context cancellation
	if ctx != nil && ctx.Err() != nil {
		return
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	if !l.h.Enabled(level) {
		return
	}

	r := &handler.Record{
		Time:      time.Now(),
		Level:     level,
		Message:   msg,
		KeyValues: keyValues,
	}

	// Handle caller detection
	skip := l.skip + delta
	if l.needsPC {
		// We call runtime.Caller() to determine the actual call site
		var pcs [1]uintptr
		// skip-1 is used because we capture the call before handler.Handle()
		if runtime.Callers(max(skip-1, 0), pcs[:]) > 0 {
			r.PC = pcs[0]
		}
	}
	if l.needsSkip && skip > 0 {
		r.Skip = skip
	}

	// Ignore handler errors (logging must not crash the application)
	_ = l.h.Handle(ctx, r)

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
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(keyValues) < 2 || l.ch == nil {
		return l
	}

	return l.withChainer(l.ch.WithAttrs(keyValues))
}

// WithGroup returns a new Logger that starts a key-value group,
// if the underlying handler supports it.
// If name is non-empty, keys of attributes will be qualified with it.
func (l *logger) WithGroup(name string) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if name == "" || l.ch == nil {
		return l
	}

	return l.withChainer(l.ch.WithGroup(name))
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

// SetLevel changes the minimum log level that will be processed if the handler supports it.
func (l *logger) SetLevel(level LogLevel) error {
	if cf := l.cf; cf != nil {
		return cf.SetLevel(level)
	}
	return nil
}

// SetOutput changes the log destination for this logger.
// If this logger was created via With/WithGroup, the output writer is shared
// with the parent logger. To create independent outputs, construct separate
// loggers with distinct handler instances.
func (l *logger) SetOutput(w io.Writer) error {
	if cf := l.cf; cf != nil {
		return cf.SetOutput(w)
	}
	return nil
}

// Sync flushes buffered log entries. Returns error on flush failure.
func (l *logger) Sync() error {
	if snc := l.snc; snc != nil {
		return snc.Sync()
	}
	return nil
}

// LogWithSkip logs a message at the given level, skipping the current caller skip value with delta.
// Use it when you need a single log entry with a different caller skip.
func (l *logger) LogWithSkip(ctx context.Context, level LogLevel, msg string, delta int, keyValues ...any) {
	l.log(ctx, level, msg, delta, keyValues...)
}

// WithCallerSkip returns a new AdvancedLogger with the caller skip set permanently.
// It returns the original logger if the skip value is unchanged.
func (l *logger) WithCallerSkip(skip int) AdvancedLogger {
	skip = max(skip, 0) + internalSkipFrames

	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.skip == skip {
		return l
	}

	// Return a clone with the same handler and new caller skip
	// if the handler does not support caller resolution
	if l.needsPC {
		return &logger{
			handlerEntry: l.handlerEntry,
			skip:         skip,
		}
	}

	adv := l.adv.WithCallerSkip(skip)
	var h handler.Handler = adv
	state := h.HandlerState()

	clone := &logger{
		handlerEntry: handlerEntry{
			h:     h,
			adv:   adv,
			state: state,
		},
		skip: skip,
	}

	// Optional interfaces
	if ch, ok := h.(handler.Chainer); ok {
		clone.ch = ch
	}
	if cf, ok := h.(handler.Configurator); ok {
		clone.cf = cf
	}
	if snc, ok := h.(handler.Syncer); ok {
		clone.snc = snc
	}

	clone.needsPC = state == nil || clone.adv == nil
	clone.needsSkip = state != nil && clone.adv != nil && state.CallerEnabled()

	return clone
}

// WithCallerSkipDelta returns a new AdvancedLogger with caller skip permanently adjusted by delta.
func (l *logger) WithCallerSkipDelta(delta int) AdvancedLogger {
	return l.WithCallerSkip(l.skip + delta)
}

// WithCaller returns a new AdvancedLogger that enables or disables caller resolution for the logger.
// It returns the original logger if the enabled value is unchanged or the handler does not support
// caller resolution. By default, caller resolution is disabled.
func (l *logger) WithCaller(enabled bool) AdvancedLogger {
	// TODO: implement
	return l
}

// WithTrace returns a new AdvancedLogger that enables or disables trace logging for the logger.
// It returns the original logger if the enabled value is unchanged or the handler does not support
// trace logging. By default, trace logging is disabled.
func (l *logger) WithTrace(enabled bool) AdvancedLogger {
	// TODO: implement
	return l
}

// WithLevel returns a new AdvancedLogger with a new minimum level applied to the handler.
// It returns the original logger if the level value is unchanged or the handler does not support
// level control.
func (l *logger) WithLevel(level LogLevel) AdvancedLogger {
	// TODO: implement
	return l
}

// WithOutput returns a new AdvancedLogger with the output writer set permanently.
// It returns the original logger if the writer value is unchanged or the handler does not support
// output control.
func (l *logger) WithOutput(w io.Writer) AdvancedLogger {
	// TODO: implement
	return l
}

// withChainer returns a new Logger with the given Chainer attached.
// It returns the original logger if the chainer value is unchanged.
// The returned logger will have the same skip value as the original logger.
func (l *logger) withChainer(ch handler.Chainer) Logger {
	if ch == nil {
		return l
	}

	var h handler.Handler = ch
	state := h.HandlerState()

	clone := &logger{
		handlerEntry: handlerEntry{
			h:     h,
			ch:    ch,
			state: state,
		},
		skip: l.skip,
	}

	// Optional interfaces
	if adv, ok := h.(handler.AdvancedHandler); ok {
		clone.adv = adv
	}
	if cf, ok := h.(handler.Configurator); ok {
		clone.cf = cf
	}
	if snc, ok := h.(handler.Syncer); ok {
		clone.snc = snc
	}

	clone.needsPC = state == nil || clone.adv == nil
	clone.needsSkip = state != nil && clone.adv != nil && state.CallerEnabled()

	return clone
}
