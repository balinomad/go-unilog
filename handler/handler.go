package handler

import (
	"context"
	"io"
	"time"
)

// Handler is the core adapter contract that all logger implementations must satisfy.
type Handler interface {
	// Handle processes a log record. Must handle nil context gracefully.
	// Returns error only for unrecoverable failures (disk full, etc.).
	Handle(ctx context.Context, record *Record) error

	// Enabled reports whether the handler processes records at the given level.
	// Called before building expensive Record objects.
	Enabled(level LogLevel) bool
}

// Chainer extends Handler with attribute and group attachment.
// Implementations return new Handler instances (immutable pattern).
type Chainer interface {
	Handler

	// WithAttrs returns a new Handler with the given attributes attached.
	WithAttrs(attrs []Attr) Handler

	// WithGroup returns a new Handler that qualifies subsequent attribute keys
	// with the group name.
	WithGroup(name string) Handler
}

// Configurator enables runtime reconfiguration of handler settings.
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

	// Attrs is the list of attributes associated with the log entry.
	Attrs []Attr

	// PC is the program counter for source location (0 if unavailable).
	// It will be used for loggers that don't support source location natively.
	PC uintptr

	// Skip is the number of stack frames to skip for source location.
	// It will be used for loggers that support source location natively.
	Skip int
}

// Attr represents a key-value pair in structured logging.
type Attr struct {
	Key   string
	Value any
}
