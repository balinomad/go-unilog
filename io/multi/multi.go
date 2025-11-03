// Package multi provides a thread-safe implementation of an io.Writer
// that duplicates its writes to all the underlying writers.
package multi

import (
	"errors"
	"io"
	"sync"
)

// MultiWriter is an io.WriteCloser that duplicates its writes to all the
// underlying writers. It is safe for concurrent use.
type MultiWriter struct {
	writers []io.Writer
	mu      sync.Mutex
}

// New creates and returns a new MultiWriter.
// It accepts one or more io.Writer instances to which it will distribute writes.
func New(writers ...io.Writer) *MultiWriter {
	// Create a defensive copy of the writers slice to prevent external modifications
	w := make([]io.Writer, len(writers))
	copy(w, writers)
	return &MultiWriter{writers: w}
}

// Write writes the byte slice p to all underlying writers.
// It returns the number of bytes written and the first error encountered, if any.
// The write operation is atomic, protected by a mutex.
func (t *MultiWriter) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return n, err
		}
		// If a partial write without an error occurs, we return it immediately
		if n != len(p) {
			return n, io.ErrShortWrite
		}
	}
	return len(p), nil
}

// Close attempts to close all underlying writers that implement the io.Closer interface.
// It continues to close all writers even after an error occurs.
// All errors encountered are combined and returned.
func (t *MultiWriter) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var errs []error
	for _, w := range t.writers {
		if c, ok := w.(io.Closer); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

// AddWriters adds one or more writers to the MultiWriter.
// This operation is thread-safe.
func (t *MultiWriter) AddWriters(writers ...io.Writer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writers = append(t.writers, writers...)
}
