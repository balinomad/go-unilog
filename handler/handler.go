package handler

import (
	"context"
	"io"
	"time"
)

// HandlerState is an immutable interface for handlers to expose their state.
type HandlerState interface {
	// CallerEnabled returns whether caller information should be included.
	CallerEnabled() bool

	// TraceEnabled returns whether stack traces should be included for error-level logs.
	TraceEnabled() bool

	// CallerSkip returns the current caller skip value.
	CallerSkip() int
}

// Handler is the core adapter contract that all logger implementations must satisfy.
type Handler interface {
	// Handle processes a log record. Must handle nil context gracefully.
	// Returns error only for unrecoverable failures (disk full, etc.).
	Handle(ctx context.Context, record *Record) error

	// Enabled reports whether the handler processes records at the given level.
	// Called before building expensive Record objects.
	Enabled(level LogLevel) bool

	// HandlerState returns an immutable HandlerState that exposes handler state.
	HandlerState() HandlerState

	// Features returns the handler's supported features.
	Features() HandlerFeatures
}

// Chainer extends Handler with methods for chaining log attributes.
// Implementations return new Chainer instances (immutable pattern).
type Chainer interface {
	Handler

	// WithAttrs returns a new Chainer with the given key-value pairs added.
	// It returns the original logger if no key-value pairs are provided.
	WithAttrs(keyValues []any) Chainer

	// WithGroup returns a new Chainer that qualifies subsequent attribute keys
	// with the group name. It returns the original logger if the name is empty.
	WithGroup(name string) Chainer
}

// AdvancedHandler extends Handler with immutable configuration methods.
// These methods mirror the 'With...' methods on the unilog.AdvancedLogger.
// Implementations return new AdvancedHandler instances (immutable pattern).
type AdvancedHandler interface {
	Handler

	// WithLevel returns a new AdvancedHandler with a new minimum level applied.
	// It returns the original logger if the level value is unchanged.
	WithLevel(level LogLevel) AdvancedHandler

	// WithOutput returns a new AdvancedHandler with the output writer set permanently.
	// It returns the original logger if the writer value is unchanged.
	WithOutput(w io.Writer) AdvancedHandler

	// WithCallerSkip returns a new AdvancedHandler with the absolute
	// user-visible caller skip set. Negative skip values are clamped to zero.
	WithCallerSkip(skip int) AdvancedHandler

	// WithCallerSkipDelta returns a new AdvancedHandler with the caller skip
	// adjusted by delta.
	// The delta is applied relative to the current skip. Zero delta is a no-op.
	// If the new skip is negative, it is clamped to zero.
	WithCallerSkipDelta(delta int) AdvancedHandler

	// WithCaller returns a clone that enables or disables caller resolution.
	// It returns the original logger if the enabled value is unchanged.
	// By default, caller resolution is disabled.
	WithCaller(enabled bool) AdvancedHandler

	// WithTrace returns a new AdvancedHandler that enables or disables trace logging.
	// It returns the original logger if the enabled value is unchanged.
	// By default, trace logging is disabled.
	WithTrace(enabled bool) AdvancedHandler
}

// Configurator enables mutable runtime reconfiguration of handler settings.
// Implementing this interface implies the Handler is mutable, allowing for
// high-speed atomic updates (e.g., level changes) without allocations.
// This performance gain comes at the cost of immutability; implementers
// must ensure these methods are safe for concurrent use.
type Configurator interface {
	// SetLevel changes the minimum log level that will be processed.
	SetLevel(level LogLevel) error

	// SetOutput changes the destination for log output.
	SetOutput(w io.Writer) error
}

// Syncer flushes any buffered log entries.
type Syncer interface {
	// Sync flushes buffered log entries. Returns error on flush failure.
	Sync() error
}

// Record represents a single log entry with structured attributes.
type Record struct {
	// Time is the timestamp of the log entry.
	Time time.Time

	// Level is the log level of the log entry.
	Level LogLevel

	// Message is the log message.
	Message string

	// KeyValues is the list of attributes associated with the log entry.
	KeyValues []any

	// PC is the program counter for source location (0 if unavailable).
	// It will be used for loggers that don't support source location natively.
	PC uintptr

	// Skip is the number of stack frames to skip for source location.
	// It will be used for loggers that support source location natively.
	Skip int
}
