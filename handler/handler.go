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

// Cloner enables deep copying of handlers.
type Cloner interface {
	// Clone returns a deep copy of the handler.
	Clone() Handler
}

// Record represents a single log entry with structured attributes.
type Record struct {
	Time    time.Time
	Level   LogLevel
	Message string
	Attrs   []Attr
	PC      uintptr // Program counter for source location (0 if unavailable)
}

// Attr represents a key-value pair in structured logging.
type Attr struct {
	Key   string
	Value any
}
