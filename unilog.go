// Package unilog provides a unified logging interface for structured and leveled
// logging. The API is designed to be easy to use and to work with a variety of
// loggers, such as log15, logrus, zerolog, zap and Go Kit's log and slog packages.
package unilog

import (
	"context"
	"io"

	"github.com/balinomad/go-unilog/handler"
)

// Re-export type so users only import unilog.
type LogLevel = handler.LogLevel

// Re-export constants.
const (
	TraceLevel    LogLevel = handler.TraceLevel
	DebugLevel    LogLevel = handler.DebugLevel
	InfoLevel     LogLevel = handler.InfoLevel
	WarnLevel     LogLevel = handler.WarnLevel
	ErrorLevel    LogLevel = handler.ErrorLevel
	CriticalLevel LogLevel = handler.CriticalLevel
	FatalLevel    LogLevel = handler.FatalLevel
	PanicLevel    LogLevel = handler.PanicLevel
)

// Re-export errors.
var (
	ErrInvalidLogLevel   error = handler.ErrInvalidLogLevel
	ErrAtomicWriterFail  error = handler.ErrAtomicWriterFail
	ErrFailedOption      error = handler.ErrFailedOption
	ErrInvalidFormat     error = handler.ErrInvalidFormat
	ErrInvalidSourceSkip error = handler.ErrInvalidSourceSkip
	ErrNilWriter         error = handler.ErrNilWriter
)

// CoreLogger is the core logging interface used internally by log adapters.
// It unifies structured and leveled logging across multiple backends.
// Context is always passed but may be ignored by implementations
// that do not support context-aware logging.
type CoreLogger interface {
	// Log is the generic logging entry point.
	// All level-specific methods (Debug, Info, etc.) delegate to this.
	// Logging on Fatal and Panic levels will exit the process.
	Log(ctx context.Context, level LogLevel, msg string, keyValues ...any)

	// Enabled reports whether logging at the given level is currently enabled.
	Enabled(level LogLevel) bool
}

// Logger is the main logging interface.
// It provides convenience methods for logging at specific levels and with groups.
type Logger interface {
	CoreLogger

	// With returns a new Logger that always includes the given key-value pairs.
	// Implementations should treat this immutably (original logger unchanged).
	With(keyValues ...any) Logger

	// WithGroup returns a new Logger that starts a key-value group.
	// If name is non-empty, keys of attributes will be qualified with it.
	WithGroup(name string) Logger

	// Convenience level-specific methods (all call Log under the hood).
	Trace(ctx context.Context, msg string, keyValues ...any)    // Logs at the trace level
	Debug(ctx context.Context, msg string, keyValues ...any)    // Logs at the debug level
	Info(ctx context.Context, msg string, keyValues ...any)     // Logs at the info level
	Warn(ctx context.Context, msg string, keyValues ...any)     // Logs at the warn level
	Error(ctx context.Context, msg string, keyValues ...any)    // Logs at the error level
	Critical(ctx context.Context, msg string, keyValues ...any) // Logs at the critical level
	Fatal(ctx context.Context, msg string, keyValues ...any)    // Logs at the fatal level and exits the process
	Panic(ctx context.Context, msg string, keyValues ...any)    // Logs at the panic level and panics
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
	// LogWithSkip logs a message at the given level, skipping the given number of stack frames.
	LogWithSkip(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any)

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
