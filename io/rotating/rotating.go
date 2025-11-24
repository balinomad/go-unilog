// Package rotating provides a safe, durable, size-based log file writer with automatic rotation.
//
// Purpose
//
//	rotating.RotatingWriter is a simple, concurrent-safe writer that appends to a file
//	and rotates it when it grows beyond a configured size. Rotation is performed
//	durably (fsync on files and best-effort directory sync) using an atomic-style
//	swap so that after rotation there is always a usable active file.
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
//     current file before rotation, fsync() the new temporary file, perform atomic
//     renames, and fsync() the containing directory (best-effort). Exact guarantees
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
	"strconv"
	"strings"
	"sync"
)

// options holds the configuration for a RotatingWriter.
type options struct {
	maxSizeMB  int         // 0 => no size-based rotation
	maxBackups int         // 0 => do not keep backups (rotated file removed)
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
type RotatingWriter struct {
	mu          sync.Mutex
	filename    string
	maxSize     int64 // bytes; 0 => no rotation
	maxBackups  int   // 0 => no cleanup
	file        io.WriteCloser
	currentSize int64
	errHandler  func(error) // optional error handler, fallback to stderr
}

// Ensure interface conformance.
var _ io.WriteCloser = (*RotatingWriter)(nil)

// New constructs a RotatingWriter.
// filename must be non-empty. Options customize behavior; defaults:
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

	// If rotation is needed before writing, try to rotate
	if w.maxSize > 0 && w.currentSize+int64(len(p)) > w.maxSize {
		if rerr := w.rotate(); rerr != nil {
			// Rotation failed: ensure file handle exists before attempting write
			if w.file == nil {
				if oerr := w.openExistingOrNew(); oerr != nil {
					// Can't reopen: return rotation and reopen errors
					return 0, errors.Join(
						fmt.Errorf("rotation failed: %w", rerr),
						fmt.Errorf("reopen failed: %w", oerr))
				}
			}
			// File handle exists: proceed with write despite rotation failure
			w.report(fmt.Errorf("rotation failed: %w", rerr))
		}
	}

	// Panic guard: should never happen if rotate/openExistingOrNew work correctly
	if w.file == nil {
		return 0, fmt.Errorf("internal error: file handle is nil")
	}

	n, err = w.file.Write(p)
	if n > 0 {
		w.currentSize += int64(n)
	}
	return n, err
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

// openExistingOrNew opens the active file for appending or creates it if it does not exist.
// Caller must hold the lock.
func (w *RotatingWriter) openExistingOrNew() error {
	// Ensure the directory exists
	dir := filepath.Dir(w.filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get file info to check current size
	info, err := os.Stat(w.filename)
	if err == nil {
		// File exists, set current size
		w.currentSize = info.Size()
	} else if os.IsNotExist(err) {
		w.currentSize = 0
	} else {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Open the file for writing, create if it doesn't exist, and append
	f, err := os.OpenFile(w.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	w.file = f

	return nil
}

// rotate performs a durable atomic-style rotation while lock is held.
// Caller must hold the lock.
// Some rotation errors are reported and the writer may still be usable.
//
// Steps:
//   - try to fsync current file (best-effort)
//   - close current file
//   - create a temporary file in same dir, fsync it and close it
//   - rename current -> .1
//   - rename tmp -> current
//   - fsync directory (best-effort)
//   - remove old backups beyond maxBackups (best-effort)
//   - reopen active file
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

	// If maxBackups is 0, just remove the current file and create a new one
	if w.maxBackups == 0 {
		if err := os.Remove(w.filename); err != nil && !os.IsNotExist(err) {
			// If we can't remove the file, try to reopen it to maintain functionality
			if reopenErr := w.openExistingOrNew(); reopenErr != nil {
				// Both remove and reopen failed: this is a serious error
				return errors.Join(
					fmt.Errorf("failed to remove log file: %w", err),
					fmt.Errorf("failed to reopen log file: %w", reopenErr))
			}
			// Remove failed but reopen succeeded: writer is functional but rotation didn't work as expected
			w.report(fmt.Errorf("failed to remove log file during rotation: %w", err))
			return nil
		}
		// Remove succeeded (or file didn't exist), now create new file
		return w.openExistingOrNew()
	}

	// Rotate existing backups in reverse order to avoid overwriting
	for i := w.maxBackups; i > 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.filename, i-1)
		newPath := fmt.Sprintf("%s.%d", w.filename, i)

		// Check if the old backup file exists before trying to rename it
		if _, err := os.Stat(oldPath); err == nil {
			if err := safeRename(oldPath, newPath); err != nil {
				// Non-critical error: log it and continue
				w.report(fmt.Errorf("failed to rotate backup file %s: %w", oldPath, err))
			}
		}
	}

	// Rename the current log file to a backup name
	backupFilename := w.filename + ".1"
	if err := safeRename(w.filename, backupFilename); err != nil {
		// If rename fails, try to reopen the original file to avoid losing logs
		_ = w.openExistingOrNew()
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Remove backups that exceed the maxBackups limit
	if w.maxBackups > 0 {
		if err := removeExtraBackups(w.filename, w.maxBackups); err != nil {
			// Non-fatal: rotation succeeded, but cleanup failed
			w.report(fmt.Errorf("rotation succeeded but cleanup failed: %w", err))
		}
	}

	return w.openExistingOrNew()
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

// removeExtraBackups removes rotated files with numeric suffixes beyond maxBackups.
// Files expected to be named "<basename>.N" where N is a positive integer.
func removeExtraBackups(fullpath string, maxBackups int) error {
	dir := filepath.Dir(fullpath)
	base := filepath.Base(fullpath)

	ents, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory for cleanup: %w", err)
	}

	type entry struct {
		name  string
		index int
		path  string
	}

	var entries []entry
	prefix := base + "."

	// Find all files with the expected prefix and numeric suffix
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		idxStr := name[len(prefix):]
		idx, err := strconv.Atoi(idxStr)
		if err != nil || idx < 1 {
			continue
		}
		entries = append(entries, entry{
			name:  name,
			index: idx,
			path:  filepath.Join(dir, name),
		})
	}

	if len(entries) <= maxBackups {
		return nil
	}

	// Sort by numeric index ascending (.1 is newest, higher numbers are older)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].index < entries[j].index
	})

	// Remove entries beyond maxBackups (highest-indexed/oldest files)
	var errs []error
	for _, e := range entries[maxBackups:] {
		if rmErr := os.Remove(e.path); rmErr != nil && !os.IsNotExist(rmErr) {
			errs = append(errs, fmt.Errorf("failed to remove %s: %w", e.name, rmErr))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

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
