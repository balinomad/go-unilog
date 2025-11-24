// Package rotating provides a safe, durable, size-based log file writer with automatic rotation.
//
// Purpose
//
//	rotating.RotatingWriter is a simple, concurrent-safe writer that appends to a file
//	and rotates it when it grows beyond a configured size. Rotation is performed
//	durably (fsync on files) using atomic renames so that after rotation there is
//	always a usable active file.
//
// Intended use
//   - General-purpose application logging where rotations are relatively infrequent
//     (minutes/hours under normal configs). This implementation favors correctness
//     and crash-safety over micro-optimizations for pathological high-frequency rotation.
//   - If you require extremely high throughput with rotations happening many times
//     per second, use a specialized async logger or an in-process aggregator; this
//     package intentionally does not optimize for that scenario.
//
// Guarantees & limitations
//   - Rotation is durable on typical POSIX filesystems: we attempt to fsync() the
//     current file before rotation, then perform atomic renames. Exact guarantees
//     may vary with underlying filesystem semantics (NFS, overlay filesystems, and
//     some Windows filesystems differ).
//   - The writer is safe for multiple goroutines calling Write concurrently.
//   - Non-fatal internal problems (cleanup failures, non-critical I/O errors) are
//     reported asynchronously via the provided error handler or printed to stderr.
//   - There is no configuration toggle for "durable vs fast"; this package uses the
//     durable rotation strategy by design.
//   - The API is intentionally small and opinionated: configure with functional
//     options and use New(...) to create a writer.
package rotating

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// options holds the configuration for a RotatingWriter.
type options struct {
	maxSizeMB  int         // 0 => no size-based rotation
	maxBackups int         // 0 => keep all backups (no cleanup)
	errHandler func(error) // optional non-fatal error handler
}

// Option sets optional configuration for New.
type Option func(*options)

// WithMaxSizeMB sets the maximum file size in megabytes before rotation.
// Zero disables size-based rotation. Must be non-negative.
func WithMaxSizeMB(mb int) Option {
	return func(o *options) {
		o.maxSizeMB = mb
	}
}

// WithMaxBackups sets how many rotated backups to retain.
// Zero means keep all rotated files. Must be non-negative.
func WithMaxBackups(n int) Option {
	return func(o *options) {
		o.maxBackups = n
	}
}

// WithErrorHandler sets an optional handler for non-fatal internal errors.
// The handler will be called asynchronously and must not call back into this writer.
// If nil, internal problems are printed to os.Stderr.
func WithErrorHandler(h func(error)) Option {
	return func(o *options) {
		o.errHandler = h
	}
}

// RotatingWriter is an io.WriteCloser that rotates log files when they reach a specified size.
// It is safe for concurrent use by multiple goroutines.
//
// Fields are not exported and should not be accessed directly.
// Use the provided methods to interact with the writer.
type RotatingWriter struct {
	mu          sync.Mutex     // Protects all mutable state
	filename    string         // Active log file path
	maxSize     int64          // bytes; 0 => no rotation
	maxBackups  int            // 0 => no cleanup
	file        io.WriteCloser // Active log file handle
	currentSize int64          // Current file size in bytes
	errHandler  func(error)    // Optional error handler, fallback to stderr
}

// Ensure interface conformance.
var _ io.WriteCloser = (*RotatingWriter)(nil)

// New constructs a RotatingWriter.
// filename must be non-empty. Options customize behavior.
// Defaults: no size-based rotation, 7 backups retained, errors to stderr.
func New(filename string, opts ...Option) (*RotatingWriter, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename cannot be empty")
	}

	o := &options{
		maxSizeMB:  0,
		maxBackups: 7,
		errHandler: nil,
	}

	for _, opt := range opts {
		opt(o)
	}
	if o.maxSizeMB < 0 {
		return nil, fmt.Errorf("max size must be non-negative")
	}
	if o.maxBackups < 0 {
		return nil, fmt.Errorf("max backups must be non-negative")
	}

	w := &RotatingWriter{
		filename:   filename,
		maxSize:    int64(o.maxSizeMB) * 1024 * 1024,
		maxBackups: o.maxBackups,
		errHandler: o.errHandler,
	}

	if err := w.openExistingOrNew(); err != nil {
		return nil, err
	}
	return w, nil
}

// Write appends p to the active file. If the write would exceed maximum size,
// rotation is attempted first. Write is safe for concurrent callers.
func (w *RotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Reject writes after Close
	if w.file == nil {
		return 0, fmt.Errorf("write attempt on closed file")
	}

	// If rotation is needed before writing, try to rotate
	if w.maxSize > 0 && w.currentSize+int64(len(p)) > w.maxSize {
		if rerr := w.rotate(); rerr != nil {
			// Rotation failed: ensure file handle exists before attempting write
			if w.file == nil {
				if oerr := w.openExistingOrNew(); oerr != nil {
					// Can't reopen: return rotation and reopen errors
					return 0, errors.Join(
						fmt.Errorf("reopen failed: %w", oerr),
						fmt.Errorf("rotation failed: %w", rerr))
				}
			}
			// File handle exists: proceed with write despite rotation failure
			w.report(fmt.Errorf("rotation failed: %w", rerr))
		}
	}

	n, err = w.file.Write(p)
	if err != nil {
		// Don't update currentSize on error to avoid inflating it
		return n, err
	}
	w.currentSize += int64(n)

	return n, nil
}

// Rotate triggers log file rotation manually. It is safe to call multiple times.
// It returns an error if rotation fails.
// Some rotation errors are reported and the writer may still be usable.
func (w *RotatingWriter) Rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.rotate()
}

// Close closes the underlying file. Safe to call multiple times.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.close()
}

// close closes the file. Caller must hold the lock.
func (w *RotatingWriter) close() error {
	if w.file == nil {
		return nil
	}

	err := w.file.Close()
	w.file = nil

	return err
}

// rotate performs a durable atomic-style rotation while lock is held.
// Caller must hold the lock.
// Some rotation errors are reported and the writer may still be usable.
//
// Steps:
//   - try to fsync current file (best-effort)
//   - close current file
//   - rename current -> X.TIMESTAMP
//   - create new active file
//   - trigger async cleanup if maxBackups > 0
func (w *RotatingWriter) rotate() error {
	// Best-effort sync current file
	if err := w.trySync(); err != nil {
		// Non-fatal: report but don't abort rotation
		w.report(fmt.Errorf("fsync before rotation failed: %w", err))
	}

	// Close current file so it can be renamed
	if err := w.close(); err != nil {
		return fmt.Errorf("failed to close file before rotation: %w", err)
	}

	// Generate timestamp-based backup filename.
	// Collision risk is negligible in practice: requires rotating twice within
	// the same microsecond on the same machine, which is prevented by the serial
	// nature of rotate() under mutex lock.
	timestamp := time.Now().Format("2006-01-02T15-04-05.000000")
	backupFilename := fmt.Sprintf("%s.%s", w.filename, timestamp)

	// Rename current file to timestamped backup
	if err := safeRename(w.filename, backupFilename); err != nil {
		// If rename fails, try to reopen the original file to avoid losing logs
		if reopenErr := w.openExistingOrNew(); reopenErr != nil {
			return errors.Join(
				fmt.Errorf("failed to reopen log file: %w", reopenErr),
				fmt.Errorf("failed to rename log file: %w", err))
		}
		// Reopen succeeded but rotation failed
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Trigger async cleanup if maxBackups > 0
	if w.maxBackups > 0 {
		go w.cleanup()
	}

	return w.openExistingOrNew()
}

// cleanup removes old backup files beyond maxBackups limit.
// Must be called asynchronously to avoid blocking Write.
func (w *RotatingWriter) cleanup() {
	w.mu.Lock()
	filename := w.filename
	maxBackups := w.maxBackups
	w.mu.Unlock()

	dir := filepath.Dir(filename)
	base := filepath.Base(filename)
	prefix := base + "."

	ents, err := os.ReadDir(dir)
	if err != nil {
		w.report(fmt.Errorf("cleanup failed to read directory: %w", err))
		return
	}

	type backup struct {
		name      string
		timestamp string
	}

	var backups []backup

	// Find all backup files with valid timestamp suffixes
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}

		suffix := name[len(prefix):]

		// Validate by attempting to parse as timestamp
		if _, err := time.Parse("2006-01-02T15-04-05.000000", suffix); err != nil {
			continue
		}

		backups = append(backups, backup{
			name:      name,
			timestamp: suffix,
		})
	}

	if len(backups) <= w.maxBackups {
		return
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].timestamp > backups[j].timestamp
	})

	// Remove oldest backups beyond maxBackups
	for _, b := range backups[maxBackups:] {
		path := filepath.Join(dir, b.name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			w.report(fmt.Errorf("cleanup failed to remove %s: %w", b.name, err))
		}
	}
}

// trySync calls Sync on the underlying *os.File if possible.
// Returns error on failure.
func (w *RotatingWriter) trySync() error {
	if w.file == nil {
		return nil
	}

	if f, ok := w.file.(*os.File); ok {
		return f.Sync()
	}

	return nil
}

// report calls the configured error handler in a goroutine to avoid blocking the writer.
// If no handler is configured, errors are printed to stderr.
func (w *RotatingWriter) report(err error) {
	if err == nil {
		return
	}

	handler := w.errHandler

	if handler == nil {
		fmt.Fprintln(os.Stderr, "rotating writer:", err)
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "rotating writer: error handler panicked: %v\n", r)
			}
		}()
		handler(err)
	}()
}

// openExistingOrNew opens the active file for appending or creates it if it does not exist.
// Caller must hold the lock.
func (w *RotatingWriter) openExistingOrNew() error {
	// Ensure the directory exists
	dir := filepath.Dir(w.filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open the file for writing, create if it doesn't exist, and append
	f, err := os.OpenFile(w.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", w.filename, err)
	}

	// Get current size from opened file
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to stat file %s: %w", w.filename, err)
	}

	w.file = f
	w.currentSize = info.Size()
	return nil
}

// safeRename is a wrapper around os.Rename that first removes the destination
// path if it already exists. This is necessary on Windows because os.Rename
// will fail if the destination path already exists.
func safeRename(oldPath, newPath string) error {
	if _, err := os.Stat(newPath); err == nil {
		if err := os.Remove(newPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing destination %s: %w", newPath, err)
		}
	}

	return os.Rename(oldPath, newPath)
}
