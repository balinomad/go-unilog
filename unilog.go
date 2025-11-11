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
	ErrOptionApplyFailed error = handler.ErrOptionApplyFailed
	ErrInvalidFormat     error = handler.ErrInvalidFormat
	ErrInvalidSourceSkip error = handler.ErrInvalidSourceSkip
	ErrNilWriter         error = handler.ErrNilWriter
)

// Logger is the main logging interface.
// It provides convenience methods for logging at specific levels and with groups.
// It unifies structured and leveled logging across multiple backends.
// Context is always passed but may be ignored by implementations
// that do not support context-aware logging.
type Logger interface {
	// Log is the generic logging entry point. It implements the Logger interface.
	// Logging on Fatal and Panic levels will exit the process.
	Log(ctx context.Context, level LogLevel, msg string, keyValues ...any)

	// Enabled reports whether logging at the given level is currently enabled.
	Enabled(level LogLevel) bool

	// With returns a new Logger that always includes the given key-value pairs.
	With(keyValues ...any) Logger

	// WithGroup returns a new Logger that starts a key-value group.
	WithGroup(name string) Logger

	// Trace is a convenience method that logs a message at the trace level.
	Trace(ctx context.Context, msg string, keyValues ...any)

	// Debug is a convenience method that logs a message at the debug level.
	Debug(ctx context.Context, msg string, keyValues ...any)

	// Info is a convenience method that logs a message at the info level.
	Info(ctx context.Context, msg string, keyValues ...any)

	// Warn is a convenience method that logs a message at the warn level.
	Warn(ctx context.Context, msg string, keyValues ...any)

	// Error is a convenience method that logs a message at the error level.
	Error(ctx context.Context, msg string, keyValues ...any)

	// Critical is a convenience method that logs a message at the critical level.
	Critical(ctx context.Context, msg string, keyValues ...any)

	// Fatal is a convenience method that logs a message at the fatal level and then exits.
	Fatal(ctx context.Context, msg string, keyValues ...any)

	// Panic is a convenience method that logs a message at the panic level and then panics.
	Panic(ctx context.Context, msg string, keyValues ...any)
}

// AdvancedLogger is an interface for loggers that support advanced features.
type AdvancedLogger interface {
	Logger

	// LogWithSkip logs a message at the given level, skipping the given number of stack frames.
	// It ignores the current caller skip value and uses the provided one.
	// Use it when you need a single log entry with a different caller skip.
	LogWithSkip(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any)

	// WithCallerSkip returns a new AdvancedLogger with the caller skip set permanently.
	// It returns the original logger if the skip value is unchanged.
	WithCallerSkip(skip int) AdvancedLogger

	// WithCallerSkipDelta returns a new AdvancedLogger with caller skip permanently adjusted by delta.
	WithCallerSkipDelta(delta int) AdvancedLogger

	// WithCaller returns a new AdvancedLogger that enables or disables caller resolution for the logger.
	// It returns the original logger if the enabled value is unchanged or the handler does not support
	// caller resolution. By default, caller resolution is disabled.
	WithCaller(enabled bool) AdvancedLogger

	// WithTrace returns a new AdvancedLogger that enables or disables trace logging for the logger.
	// It returns the original logger if the enabled value is unchanged or the handler does not support
	// trace logging. By default, trace logging is disabled.
	WithTrace(enabled bool) AdvancedLogger

	// WithLevel returns a new AdvancedLogger with a new minimum level applied to the handler.
	// It returns the original logger if the level value is unchanged or the handler does not support
	// level control.
	WithLevel(level LogLevel) AdvancedLogger

	// WithOutput returns a new AdvancedLogger with the output writer set permanently.
	// It returns the original logger if the writer value is unchanged or the handler does not support
	// output control.
	WithOutput(w io.Writer) AdvancedLogger

	// Sync flushes buffered log entries if supported by the handler. Returns error on flush failure.
	Sync() error

	/*
		Future plans:

		// WithHandler returns a new AdvancedLogger that uses the provided handler.
		// It returns the original logger if the handler value is unchanged.
		WithHandler(h handler.Handler) AdvancedLogger

		// WithHandlerOption returns a new AdvancedLogger that applies the provided option to the handler.
		// It returns the original logger if the handler value is unchanged.
		WithHandlerOption(fn func(h handler.Handler) handler.Handler) AdvancedLogger
	*/
}

// MutableLogger enables mutable runtime reconfiguration of the logger.
type MutableLogger = handler.Configurator
