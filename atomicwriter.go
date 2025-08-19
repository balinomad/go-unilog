package unilog

import (
	"io"
	"sync/atomic"
)

// AtomicWriter is a thread-safe io.Writer that allows swapping
// the underlying writer atomically without blocking concurrent writes.
type AtomicWriter struct {
	w atomic.Value
}

// Ensure AtomicWriter implements io.Writer.
var _ io.Writer = (*AtomicWriter)(nil)

// NewAtomicWriter creates an AtomicWriter with the given initial writer.
// The writer must be non-nil.
func NewAtomicWriter(w io.Writer) (*AtomicWriter, error) {
	if w == nil {
		return nil, ErrNilWriter
	}
	aw := &AtomicWriter{}
	aw.w.Store(w)
	return aw, nil
}

// Write writes to the current underlying writer atomically.
func (aw *AtomicWriter) Write(p []byte) (n int, err error) {
	w := aw.w.Load().(io.Writer)
	return w.Write(p)
}

// Swap replaces the underlying writer atomically.
// The writer must be non-nil.
func (aw *AtomicWriter) Swap(w io.Writer) error {
	if w == nil {
		return ErrNilWriter
	}
	aw.w.Store(w)
	return nil
}
