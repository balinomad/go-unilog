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

// logger wraps a handler.Handler to implement the Logger interface.
// It is thread-safe.
type logger struct {
	mu sync.RWMutex

	// Handler and cached interfaces
	h     handler.Handler
	ch    handler.Chainer
	cfg   handler.Configurable
	adj   handler.CallerAdjuster
	tog   handler.FeatureToggler
	mcfg  handler.MutableConfig
	snc   handler.Syncer
	state handler.HandlerState

	// Caller detection flags
	needsPC   bool
	needsSkip bool
	skip      int
}

// Ensure logger implements required interfaces
var (
	_ Logger         = (*logger)(nil)
	_ AdvancedLogger = (*logger)(nil)
	_ MutableLogger  = (*logger)(nil)
)

// internalSkipFrames is the number of stack frames between user call and backend.
//
// Stack frames:
//  1. User code → logger.Info() / logger.Log()
//  2. logger.Info() → logger.log()
//  3. logger.log() → handler.Handle()
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

	return newLogger(h, internalSkipFrames), nil
}

// newLogger creates a logger with specified skip offset.
func newLogger(h handler.Handler, skip int) *logger {
	// Apply skip to handler if supported
	skip = max(skip, 0)
	if adj, ok := h.(handler.CallerAdjuster); ok && skip > 0 {
		h = adj.WithCallerSkip(skip)
	}

	features := h.Features()
	state := h.HandlerState()

	l := &logger{
		h:         h,
		state:     state,
		skip:      skip,
		needsPC:   !features.Supports(handler.FeatNativeCaller),
		needsSkip: features.Supports(handler.FeatNativeCaller) && state.CallerEnabled(),
	}

	// Cache optional interfaces
	l.ch, _ = h.(handler.Chainer)
	l.cfg, _ = h.(handler.Configurable)
	l.adj, _ = h.(handler.CallerAdjuster)
	l.tog, _ = h.(handler.FeatureToggler)
	l.mcfg, _ = h.(handler.MutableConfig)
	l.snc, _ = h.(handler.Syncer)

	return l
}

// log logs a message at the given level with optional skip adjustment.
func (l *logger) log(ctx context.Context, level LogLevel, msg string, skipDelta int, keyValues ...any) {
	// Fast path: check level before allocations
	if !l.h.Enabled(level) {
		return
	}

	// Respect context cancellation
	if ctx != nil && ctx.Err() != nil {
		return
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	r := &handler.Record{
		Time:      time.Now(),
		Level:     level,
		Message:   msg,
		KeyValues: keyValues,
	}

	// Handle caller detection
	skip := l.skip + skipDelta
	if l.needsPC && skip > 0 {
		var pcs [1]uintptr
		// skip-1 accounts for this frame not being in backend
		if runtime.Callers(skip-1, pcs[:]) > 0 {
			r.PC = pcs[0]
		}
	}
	if l.needsSkip && skip > 0 {
		r.Skip = skip
	}

	// Ignore handler errors (logging must not crash application)
	_ = l.h.Handle(ctx, r)

	// Handle termination levels
	switch level {
	case FatalLevel:
		os.Exit(1)
	case PanicLevel:
		panic(msg)
	}
}

// Log is the generic logging entry point.
func (l *logger) Log(ctx context.Context, level LogLevel, msg string, keyValues ...any) {
	l.log(ctx, level, msg, 0, keyValues...)
}

// Enabled reports whether logging at the given level is enabled.
func (l *logger) Enabled(level LogLevel) bool {
	return l.h.Enabled(level)
}

// With returns a new Logger with the given key-value pairs added.
func (l *logger) With(keyValues ...any) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(keyValues) < 2 || l.ch == nil {
		return l
	}

	return l.cloneWithHandler(l.ch.WithAttrs(keyValues))
}

// WithGroup returns a new Logger that starts a key-value group.
func (l *logger) WithGroup(name string) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if name == "" || l.ch == nil {
		return l
	}

	return l.cloneWithHandler(l.ch.WithGroup(name))
}

// Trace logs a message at the trace level.
func (l *logger) Trace(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, TraceLevel, msg, 0, keyValues...)
}

// Debug logs a message at the debug level.
func (l *logger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, DebugLevel, msg, 0, keyValues...)
}

// Info logs a message at the info level.
func (l *logger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, InfoLevel, msg, 0, keyValues...)
}

// Warn logs a message at the warn level.
func (l *logger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, WarnLevel, msg, 0, keyValues...)
}

// Error logs a message at the error level.
func (l *logger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, ErrorLevel, msg, 0, keyValues...)
}

// Critical logs a message at the critical level.
func (l *logger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, CriticalLevel, msg, 0, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *logger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, FatalLevel, msg, 0, keyValues...)
}

// Panic logs a message at the panic level and panics.
func (l *logger) Panic(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, PanicLevel, msg, 0, keyValues...)
}

// --- MutableLogger Methods ---

// SetLevel changes the minimum log level if the handler supports it.
func (l *logger) SetLevel(level LogLevel) error {
	if l.mcfg != nil {
		return l.mcfg.SetLevel(level)
	}
	return nil
}

// SetOutput changes the log destination if the handler supports it.
func (l *logger) SetOutput(w io.Writer) error {
	if l.mcfg != nil {
		return l.mcfg.SetOutput(w)
	}
	return nil
}

// --- AdvancedLogger Methods ---

// LogWithSkip logs a message with additional skip adjustment.
func (l *logger) LogWithSkip(ctx context.Context, level LogLevel, msg string, skipDelta int, keyValues ...any) {
	l.log(ctx, level, msg, skipDelta, keyValues...)
}

// Sync flushes buffered log entries if the handler supports it.
func (l *logger) Sync() error {
	if l.snc != nil {
		return l.snc.Sync()
	}
	return nil
}

// WithCallerSkip returns a new logger with absolute caller skip set.
func (l *logger) WithCallerSkip(skip int) AdvancedLogger {
	skip = max(skip, 0) + internalSkipFrames

	l.mu.RLock()
	currentSkip := l.skip
	adj := l.adj
	l.mu.RUnlock()

	if currentSkip == skip {
		return l
	}

	// If handler supports caller adjustment, apply it
	if adj != nil {
		return newLogger(l.adj.WithCallerSkip(skip), skip)
	}

	// Otherwise clone with new skip (PC capture will use it)
	return newLogger(l.h, skip)
}

// WithCallerSkipDelta returns a new logger with relative caller skip adjustment.
func (l *logger) WithCallerSkipDelta(delta int) AdvancedLogger {
	l.mu.RLock()
	currentSkip := l.skip
	l.mu.RUnlock()

	return l.WithCallerSkip(currentSkip - internalSkipFrames + delta)
}

// WithCaller returns a new logger with caller reporting enabled/disabled.
func (l *logger) WithCaller(enabled bool) AdvancedLogger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.tog == nil {
		return l
	}

	return l.cloneWithHandler(l.tog.WithCaller(enabled)).(AdvancedLogger)
}

// WithTrace returns a new logger with trace logging enabled/disabled.
func (l *logger) WithTrace(enabled bool) AdvancedLogger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.tog == nil {
		return l
	}

	return l.cloneWithHandler(l.tog.WithTrace(enabled)).(AdvancedLogger)
}

// WithLevel returns a new logger with minimum level set.
func (l *logger) WithLevel(level LogLevel) AdvancedLogger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.cfg == nil {
		return l
	}

	return l.cloneWithHandler(l.cfg.WithLevel(level)).(AdvancedLogger)
}

// WithOutput returns a new logger with output writer set.
func (l *logger) WithOutput(w io.Writer) AdvancedLogger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.cfg == nil {
		return l
	}

	return l.cloneWithHandler(l.cfg.WithOutput(w)).(AdvancedLogger)
}

// --- Helper Methods ---

// cloneWithHandler creates a new logger with the given handler.
// Handler must embed handler.Handler (all extension interfaces do).
func (l *logger) cloneWithHandler(h handler.Handler) Logger {
	return newLogger(h, l.skip)
}
