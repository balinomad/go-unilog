package handler

import (
	"errors"
	"io"
	"sync/atomic"

	"github.com/balinomad/go-atomicwriter"
)

// BaseOptions holds configuration common to most handlers.
type BaseOptions struct {
	Level      LogLevel
	Output     io.Writer
	Format     string // "json", "text", "console", etc.
	WithCaller bool
	WithTrace  bool
	CallerSkip int
}

// BaseHandler provides shared functionality for handler implementations.
type BaseHandler struct {
	out   *atomicwriter.AtomicWriter
	level atomic.Int32
}

// NewBaseHandler initializes shared resources.
func NewBaseHandler(opts BaseOptions) (*BaseHandler, error) {
	aw, err := atomicwriter.NewAtomicWriter(opts.Output)
	if err != nil {
		return nil, NewAtomicWriterError(err)
	}

	h := &BaseHandler{out: aw}
	h.level.Store(int32(opts.Level))
	return h, nil
}

// Enabled reports whether the handler processes records at the given level.
func (h *BaseHandler) Enabled(level LogLevel) bool {
	return level >= LogLevel(h.level.Load())
}

// SetLevel changes the minimum level of logs that will be processed.
func (h *BaseHandler) SetLevel(level LogLevel) error {
	if err := ValidateLogLevel(level); err != nil {
		return err
	}
	h.level.Store(int32(level))
	return nil
}

// SetOutput changes the destination for log output.
func (h *BaseHandler) SetOutput(w io.Writer) error {
	err := h.out.Swap(w)
	if errors.Is(err, atomicwriter.ErrNilWriter) {
		return ErrNilWriter
	}
	if err != nil {
		return NewAtomicWriterError(err)
	}
	return nil
}

// AtomicWriter returns the underlying atomic writer.
// Handlers use this to get the thread-safe writer for backend initialization.
func (h *BaseHandler) AtomicWriter() *atomicwriter.AtomicWriter {
	return h.out
}
