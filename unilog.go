// Package unilog provides a unified logging interface for structured and leveled
// logging. The API is designed to be easy to use and to work with a variety of
// loggers, such as log15, logrus, zerolog, zap and Go Kit's log and slog packages.
package unilog

import (
	"context"
	"io"
)

// Logger is the core logging interface.
// It unifies structured and leveled logging across multiple backends.
// Context is always passed but may be ignored by implementations
// that do not support context-aware logging.
type Logger interface {
	// Log is the generic logging entry point.
	// All level-specific methods (Debug, Info, etc.) delegate to this.
	Log(ctx context.Context, level LogLevel, msg string, keyValues ...any)

	// Enabled reports whether logging at the given level is currently enabled.
	Enabled(level LogLevel) bool

	// With returns a new Logger that always includes the given key-value pairs.
	// Implementations should treat this immutably (original logger unchanged).
	With(keyValues ...any) Logger

	// WithGroup returns a new Logger that starts a key-value group.
	// If name is non-empty, keys of attributes will be qualified with it.
	WithGroup(name string) Logger

	// Convenience level-specific methods (all call Log under the hood).
	Debug(ctx context.Context, msg string, keyValues ...any)
	Info(ctx context.Context, msg string, keyValues ...any)
	Warn(ctx context.Context, msg string, keyValues ...any)
	Error(ctx context.Context, msg string, keyValues ...any)
	Critical(ctx context.Context, msg string, keyValues ...any)
	Fatal(ctx context.Context, msg string, keyValues ...any)
}

// Configurator provides dynamic reconfiguration for loggers that support it.
type Configurator interface {
	// SetLevel sets the minimum enabled log level.
	SetLevel(level LogLevel) error

	// SetOutput changes the log output destination.
	SetOutput(w io.Writer) error
}

// CallerSkipper is implemented by loggers that support adjusting caller reporting.
type CallerSkipper interface {
	// CallerSkip returns the current number of stack frames skipped.
	CallerSkip() int

	// WithCallerSkip returns a new Logger with the caller skip set.
	WithCallerSkip(skip int) (Logger, error)

	// WithCallerSkipDelta returns a new Logger with caller skip adjusted by delta.
	WithCallerSkipDelta(delta int) (Logger, error)
}

// Cloner is implemented by loggers that can be deeply copied.
type Cloner interface {
	// Clone returns a deep copy of the logger.
	Clone() Logger
}

// Syncer is implemented by loggers that support flushing buffered log entries.
type Syncer interface {
	// Sync flushes any buffered log entries.
	Sync() error
}
