package unilog

import (
	"context"
	"io"
)

// Logger is the generic logging interface. It's designed to be a common
// abstraction over various logging libraries, enabling structured and
// level-based logging.
type Logger interface {
	// Sync flushes any buffered log entries.
	Sync() error

	// Log is the core logging method. All other level-specific methods (Debug, Info, etc.)
	// are convenience wrappers around this method.
	Log(ctx context.Context, level LogLevel, msg string, keyValues ...any)

	// With adds structured context to the logger. It returns a new logger
	// instance that will include the given key-value pairs in all subsequent
	// log messages. The underlying implementation should handle this immutably.
	With(keyValues ...any) Logger

	// WithGroup returns a Logger that starts a group, if name is non-empty.
	// The keys of all attributes added to the Logger will be qualified by the given name.
	WithGroup(name string) Logger

	// SetLevel dynamically changes the minimum level of logs.
	SetLevel(level LogLevel) error

	// SetOutput changes the destination for log output.
	SetOutput(w io.Writer) error

	// CallerSkip returns the current number of stack frames being skipped.
	CallerSkip() int

	// WithCallerSkip returns a new Logger instance with the caller skip value updated.
	// The original logger is not modified.
	WithCallerSkip(skip int) (Logger, error)

	// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
	// The original logger is not modified.
	WithCallerSkipDelta(delta int) (Logger, error)

	// Enabled returns true if the logger is configured for the given level.
	Enabled(level LogLevel) bool

	// Convenience methods for logging at specific levels.
	Debug(msg string, keyValues ...any)
	Info(msg string, keyValues ...any)
	Warn(msg string, keyValues ...any)
	Error(msg string, keyValues ...any)
	Critical(msg string, keyValues ...any)
	Fatal(msg string, keyValues ...any)

	// Convenience methods for logging with a context.
	DebugCtx(ctx context.Context, msg string, keyValues ...any)
	InfoCtx(ctx context.Context, msg string, keyValues ...any)
	WarnCtx(ctx context.Context, msg string, keyValues ...any)
	ErrorCtx(ctx context.Context, msg string, keyValues ...any)
	CriticalCtx(ctx context.Context, msg string, keyValues ...any)
	FatalCtx(ctx context.Context, msg string, keyValues ...any)
}
