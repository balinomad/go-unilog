package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// RotatingWriter is an io.Writer that rotates log files when they reach a certain size.
// It is safe for concurrent use.
type RotatingWriter struct {
	mu          sync.Mutex
	filename    string // The file to write to.
	maxSize     int64  // Maximum size in bytes before rotation.
	maxBackups  int    // Maximum number of old log files to retain.
	file        *os.File
	currentSize int64
}

// Ensure RotatingWriter implements io.WriteCloser for complete file handling.
var _ io.WriteCloser = (*RotatingWriter)(nil)

// RotatingWriterConfig holds the configuration for a RotatingWriter.
type RotatingWriterConfig struct {
	// Filename is the file to write logs to.
	Filename string
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	MaxSize int
	// MaxBackups is the maximum number of old log files to retain.
	MaxBackups int
}

// NewRotatingWriter creates a new RotatingWriter.
func NewRotatingWriter(config *RotatingWriterConfig) (*RotatingWriter, error) {
	if config.Filename == "" {
		return nil, fmt.Errorf("filename cannot be empty")
	}
	if config.MaxSize < 0 {
		return nil, fmt.Errorf("max size must be non-negative")
	}
	if config.MaxBackups < 0 {
		return nil, fmt.Errorf("max backups must be non-negative")
	}

	w := &RotatingWriter{
		filename:   config.Filename,
		maxSize:    int64(config.MaxSize) * 1024 * 1024, // Convert MB to bytes
		maxBackups: config.MaxBackups,
	}

	err := w.openExistingOrNew()
	if err != nil {
		return nil, err
	}
	return w, nil
}

// Write implements the io.Writer interface.
func (w *RotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if rotation is needed before writing
	if w.maxSize > 0 && w.currentSize+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			// If rotation fails, we still try to write to the current file
			// to avoid losing the log message.
			return w.file.Write(p)
		}
	}

	n, err = w.file.Write(p)
	w.currentSize += int64(n)

	return n, err
}

// Close implements the io.Closer interface.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.close()
}

// close closes the file and must be called with the lock held.
func (w *RotatingWriter) close() error {
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// openExistingOrNew opens the log file. If the file exists, it appends to it.
// If it does not exist, it creates it.
func (w *RotatingWriter) openExistingOrNew() error {
	// Ensure the directory exists
	dir := filepath.Dir(w.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Get file info to check current size
	info, err := os.Stat(w.filename)
	if err == nil {
		// File exists, set current size
		w.currentSize = info.Size()
	} else if !os.IsNotExist(err) {
		// Another error occurred (e.g., permission denied)
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Open the file for writing, create if it doesn't exist, and append
	f, err := os.OpenFile(w.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	w.file = f

	return nil
}

// rotate performs the log file rotation.
// This function must be called with the lock held.
func (w *RotatingWriter) rotate() error {
	// Close the current file
	if err := w.close(); err != nil {
		return err
	}

	// If maxBackups is 0, just remove the current file and create a new one
	if w.maxBackups == 0 {
		if err := os.Remove(w.filename); err != nil && !os.IsNotExist(err) {
			// If we can't remove the file, try to reopen it to maintain functionality
			if reopenErr := w.openExistingOrNew(); reopenErr != nil {
				// Both remove and reopen failed: this is a serious error
				return fmt.Errorf("failed to remove log file: %w, and failed to reopen: %v", err, reopenErr)
			}
			// Remove failed but reopen succeeded: writer is functional but rotation didn't work as expected
			return fmt.Errorf("failed to remove log file for rotation: %w", err)
		}
		// Remove succeeded (or file didn't exist), now create new file
		return w.openExistingOrNew()
	}

	// Rotate existing backups in reverse order to avoid overwriting files
	for i := w.maxBackups; i > 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.filename, i-1)
		newPath := fmt.Sprintf("%s.%d", w.filename, i)

		// Check if the old backup file exists before trying to rename it
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				// Log this error but continue, as it's not critical
				fmt.Fprintf(os.Stderr, "failed to rotate backup file %s: %v\n", oldPath, err)
			}
		}
	}

	// Rename the current log file to a backup name
	backupFilename := w.filename + ".1"
	if err := os.Rename(w.filename, backupFilename); err != nil {
		// If rename fails, try to reopen the original file to avoid losing logs
		_ = w.openExistingOrNew()
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Remove backups that exceed the maxBackups limit
	if w.maxBackups > 0 {
		// Find all backup files
		files, err := filepath.Glob(w.filename + ".*")
		if err == nil {
			// Sort by number to ensure we remove the oldest ones
			slices.Sort(files)

			// If we have more backups than allowed, remove the oldest.
			// This logic is simplified; a robust version would parse backup numbers.
			// The primary cleanup is handled by the loop above. This is a safety net.
			if len(files) > w.maxBackups {
				for _, f := range files[w.maxBackups:] {
					os.Remove(f)
				}
			}
		}
	}

	return w.openExistingOrNew()
}
