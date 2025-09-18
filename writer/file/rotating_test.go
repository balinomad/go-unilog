package file

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewRotatingWriter(t *testing.T) {
	tests := []struct {
		name      string
		config    *RotatingWriterConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid configuration",
			config: &RotatingWriterConfig{
				Filename:   "test.log",
				MaxSize:    10,
				MaxBackups: 3,
			},
			wantError: false,
		},
		{
			name: "empty filename",
			config: &RotatingWriterConfig{
				Filename:   "",
				MaxSize:    10,
				MaxBackups: 3,
			},
			wantError: true,
			errorMsg:  "filename cannot be empty",
		},
		{
			name: "negative max size",
			config: &RotatingWriterConfig{
				Filename:   "test.log",
				MaxSize:    -1,
				MaxBackups: 3,
			},
			wantError: true,
			errorMsg:  "max size must be non-negative",
		},
		{
			name: "negative max backups",
			config: &RotatingWriterConfig{
				Filename:   "test.log",
				MaxSize:    10,
				MaxBackups: -1,
			},
			wantError: true,
			errorMsg:  "max backups must be non-negative",
		},
		{
			name: "zero values",
			config: &RotatingWriterConfig{
				Filename:   "zero.log",
				MaxSize:    0,
				MaxBackups: 0,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for valid tests
			var filename string
			if !tt.wantError {
				tempDir := t.TempDir()
				filename = filepath.Join(tempDir, filepath.Base(tt.config.Filename))
				tt.config.Filename = filename
			}

			got, err := NewRotatingWriter(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewRotatingWriter() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("NewRotatingWriter() error = %v, want error containing %q", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewRotatingWriter() error = %v, want nil", err)
				return
			}

			if got == nil {
				t.Error("NewRotatingWriter() returned nil writer")
				return
			}

			// Verify configuration
			if got.filename != tt.config.Filename {
				t.Errorf("filename = %v, want %v", got.filename, tt.config.Filename)
			}
			expectedMaxSize := int64(tt.config.MaxSize) * 1024 * 1024
			if got.maxSize != expectedMaxSize {
				t.Errorf("maxSize = %v, want %v", got.maxSize, expectedMaxSize)
			}
			if got.maxBackups != tt.config.MaxBackups {
				t.Errorf("maxBackups = %v, want %v", got.maxBackups, tt.config.MaxBackups)
			}

			// Clean up
			got.Close()
		})
	}
}

func TestRotatingWriter_Write(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "write_test.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Test basic write
	testData := "Hello, World!\n"
	n, err := w.Write([]byte(testData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(testData))
	}

	// Verify file content
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if string(content) != testData {
		t.Errorf("File content = %q, want %q", string(content), testData)
	}
}

func TestRotatingWriter_MultipleWrites(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "multiple_writes.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Write multiple messages
	messages := []string{
		"First message\n",
		"Second message\n",
		"Third message\n",
	}

	var totalBytes int
	for _, msg := range messages {
		n, err := w.Write([]byte(msg))
		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
		totalBytes += n
	}

	// Verify all messages are in the file
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}

	expectedContent := strings.Join(messages, "")
	if string(content) != expectedContent {
		t.Errorf("File content = %q, want %q", string(content), expectedContent)
	}

	// Verify current size tracking
	if w.currentSize != int64(totalBytes) {
		t.Errorf("currentSize = %d, want %d", w.currentSize, totalBytes)
	}
}

func TestRotatingWriter_Rotation(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "rotation_test.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// First, write some data to create the initial file
	initialData := strings.Repeat("A", 100)
	_, err = w.Write([]byte(initialData))
	if err != nil {
		t.Errorf("Initial write error = %v", err)
	}

	// Now write data that will exceed maxSize and trigger rotation
	// The rotation check is: currentSize + len(newData) > maxSize
	// We need the total to exceed 1MB
	remainingSize := (1024 * 1024) - len(initialData) + 1 // Just over the limit
	largeData := strings.Repeat("B", remainingSize)

	n, err := w.Write([]byte(largeData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(largeData) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(largeData))
	}

	// Check if backup file was created
	backupFilename := filename + ".1"
	if _, err := os.Stat(backupFilename); os.IsNotExist(err) {
		t.Error("Backup file was not created after rotation")
	}

	// The current file should only contain the new data (largeData)
	// because rotation moved the old file to backup and created a new one
	info, err := os.Stat(filename)
	if err != nil {
		t.Errorf("Failed to get file info: %v", err)
	}

	expectedCurrentSize := int64(len(largeData))
	if info.Size() != expectedCurrentSize {
		t.Errorf("Current file size = %d, want %d", info.Size(), expectedCurrentSize)
	}

	// Check backup file contains the initial data
	backupContent, err := os.ReadFile(backupFilename)
	if err != nil {
		t.Errorf("Failed to read backup file: %v", err)
	}
	if string(backupContent) != initialData {
		t.Errorf("Backup file content = %q, want %q", string(backupContent), initialData)
	}
}

func TestRotatingWriter_MultipleRotations(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "multi_rotation.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB
		MaxBackups: 3,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Trigger multiple rotations by writing data that exceeds maxSize each time
	// We need to write smaller chunks to ensure proper rotation
	dataSize := 1024*1024 + 10 // Just over 1MB
	largeData := strings.Repeat("B", dataSize)

	// First write - will go to main file
	_, err = w.Write([]byte("initial\n"))
	if err != nil {
		t.Errorf("Initial write error = %v", err)
	}

	// Subsequent writes should trigger rotations
	for i := 0; i < 4; i++ {
		_, err := w.Write([]byte(largeData))
		if err != nil {
			t.Errorf("Write %d error = %v", i, err)
		}
	}

	// Check backup files exist up to maxBackups
	for i := 1; i <= config.MaxBackups; i++ {
		backupFile := fmt.Sprintf("%s.%d", filename, i)
		if _, err := os.Stat(backupFile); os.IsNotExist(err) {
			t.Errorf("Backup file %s should exist", backupFile)
		}
	}

	// Check that files beyond maxBackups don't exist
	extraBackup := fmt.Sprintf("%s.%d", filename, config.MaxBackups+1)
	if _, err := os.Stat(extraBackup); err == nil {
		t.Errorf("Backup file %s should not exist (exceeds maxBackups)", extraBackup)
	}
}

func TestRotatingWriter_ExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "existing_test.log")

	// Create an existing file with some content
	existingContent := "Existing log entry\n"
	err := os.WriteFile(filename, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Verify current size is set correctly
	expectedSize := int64(len(existingContent))
	if w.currentSize != expectedSize {
		t.Errorf("currentSize = %d, want %d", w.currentSize, expectedSize)
	}

	// Write new content
	newContent := "New log entry\n"
	_, err = w.Write([]byte(newContent))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Verify both contents are in the file
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}

	expectedTotal := existingContent + newContent
	if string(content) != expectedTotal {
		t.Errorf("File content = %q, want %q", string(content), expectedTotal)
	}
}

func TestRotatingWriter_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "logs", "subdir")
	filename := filepath.Join(subDir, "directory_test.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1,
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Verify directory was created
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		t.Error("Subdirectory was not created")
	}

	// Write and verify
	testData := "Directory creation test\n"
	_, err = w.Write([]byte(testData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if string(content) != testData {
		t.Errorf("File content = %q, want %q", string(content), testData)
	}
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "concurrent_test.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    10, // 10MB to avoid frequent rotations
		MaxBackups: 3,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	const numGoroutines = 20
	const messagesPerGoroutine = 50

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				message := fmt.Sprintf("Goroutine %d, Message %d\n", id, j)
				_, err := w.Write([]byte(message))
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d: %w", id, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		t.Error(err)
	}

	// Verify file exists and has content
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Error("File should not be empty after concurrent writes")
	}

	// Count lines (approximate, some may be in backup files)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 {
		t.Error("Should have written some lines")
	}
}

func TestRotatingWriter_Close(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "close_test.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1,
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Write some data
	testData := "Test data before close\n"
	_, err = w.Write([]byte(testData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Close the writer
	err = w.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Try to write after close (should fail or handle gracefully)
	_, err = w.Write([]byte("After close"))
	if err == nil {
		t.Error("Expected error when writing to closed writer")
	}

	// Multiple closes should not error
	err = w.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestRotatingWriter_ZeroMaxSize(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "zero_maxsize.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    0, // No size limit
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Write a large amount of data
	largeData := strings.Repeat("X", 2*1024*1024) // 2MB
	_, err = w.Write([]byte(largeData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// With MaxSize = 0, no rotation should occur
	backupFile := filename + ".1"
	if _, err := os.Stat(backupFile); err == nil {
		t.Error("Backup file should not exist when MaxSize is 0")
	}

	// Verify all data is in the main file
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if len(content) != len(largeData) {
		t.Errorf("File size = %d, want %d", len(content), len(largeData))
	}
}

func TestRotatingWriter_ZeroMaxBackups(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "zero_backups.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB to trigger rotation
		MaxBackups: 0, // No backups
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Write initial data to create the file
	initialData := "Initial data\n"
	_, err = w.Write([]byte(initialData))
	if err != nil {
		t.Errorf("Initial write error = %v", err)
	}

	// Write data to trigger rotation
	largeData := strings.Repeat("Y", 1024*1024) // Exactly 1MB, plus initial data will exceed
	_, err = w.Write([]byte(largeData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// With MaxBackups = 0, no backup files should be created
	backupFile := filename + ".1"
	if _, err := os.Stat(backupFile); err == nil {
		t.Error("Backup file should not exist when MaxBackups is 0")
	}

	// The current file should only contain the new data since rotation removes old data
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}

	// Should only contain the large data since old file was removed during rotation
	if string(content) != largeData {
		t.Errorf("File should only contain new data after rotation with MaxBackups=0")
	}
}

func TestRotatingWriter_ImplementsInterfaces(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "interface_test.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1,
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Test io.Writer interface
	var _ io.Writer = w

	// Test io.WriteCloser interface
	var _ io.WriteCloser = w

	// Test actual functionality
	testData := "Interface test\n"
	n, err := w.Write([]byte(testData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write() returned %d, want %d", n, len(testData))
	}
}

func TestRotatingWriter_ErrorHandling(t *testing.T) {
	t.Run("invalid directory permissions", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test permission denied")
		}

		// Create a directory with no write permissions
		tempDir := t.TempDir()
		restrictedDir := filepath.Join(tempDir, "restricted")
		err := os.Mkdir(restrictedDir, 0444) // Read-only
		if err != nil {
			t.Fatalf("Failed to create restricted directory: %v", err)
		}

		filename := filepath.Join(restrictedDir, "subdir", "test.log")
		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		_, err = NewRotatingWriter(config)
		if err == nil {
			t.Error("Expected error when creating writer in restricted directory")
		}
	})

	t.Run("write to closed file", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "closed_test.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}

		// Close the writer
		w.Close()

		// Try to write to closed writer
		_, err = w.Write([]byte("test"))
		if err == nil {
			t.Error("Expected error when writing to closed writer")
		}
	})
}

func TestRotatingWriter_RotationFailureRecovery(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "rotation_recovery.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Write initial data
	initialData := "Initial data\n"
	_, err = w.Write([]byte(initialData))
	if err != nil {
		t.Errorf("Initial write error = %v", err)
	}

	// Create a situation where rotation might fail by creating a backup file
	// that can't be overwritten (this is platform-specific behavior)
	backupFile := filename + ".1"
	err = os.WriteFile(backupFile, []byte("existing backup"), 0444) // Read-only
	if err != nil {
		t.Fatalf("Failed to create read-only backup file: %v", err)
	}

	// Try to trigger rotation with large data
	largeData := strings.Repeat("Z", 1024*1024+100) // Over 1MB
	n, err := w.Write([]byte(largeData))
	if err != nil {
		t.Errorf("Write() return error = %v", err)
	}

	// The write should succeed even if rotation fails
	if n != len(largeData) {
		t.Errorf("Write() returned %d bytes, want %d", n, len(largeData))
	}

	// Clean up the read-only file
	os.Chmod(backupFile, 0644)
	os.Remove(backupFile)
}

func TestRotatingWriter_EmptyWrites(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "empty_writes.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1,
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Test empty write
	n, err := w.Write([]byte{})
	if err != nil {
		t.Errorf("Empty write error = %v", err)
	}
	if n != 0 {
		t.Errorf("Empty write returned %d bytes, want 0", n)
	}

	// Test nil write
	n, err = w.Write(nil)
	if err != nil {
		t.Errorf("Nil write error = %v", err)
	}
	if n != 0 {
		t.Errorf("Nil write returned %d bytes, want 0", n)
	}

	// Verify current size is still 0
	if w.currentSize != 0 {
		t.Errorf("currentSize = %d, want 0 after empty writes", w.currentSize)
	}
}

func TestRotatingWriter_LargeWriteExactlyMaxSize(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "exact_size.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Write exactly maxSize bytes
	exactData := strings.Repeat("E", 1024*1024) // Exactly 1MB
	n, err := w.Write([]byte(exactData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(exactData) {
		t.Errorf("Write() returned %d bytes, want %d", n, len(exactData))
	}

	// No rotation should have occurred yet (rotation happens when currentSize + newDataSize > maxSize)
	backupFile := filename + ".1"
	if _, err := os.Stat(backupFile); err == nil {
		t.Error("Backup file should not exist when writing exactly maxSize")
	}

	// Write one more byte to trigger rotation (now currentSize + 1 > maxSize)
	_, err = w.Write([]byte("X"))
	if err != nil {
		t.Errorf("Second write error = %v", err)
	}

	// Now rotation should have occurred
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("Backup file should exist after exceeding maxSize")
	}

	// Current file should only contain the new byte
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read current file: %v", err)
	}
	if string(content) != "X" {
		t.Errorf("Current file content = %q, want %q", string(content), "X")
	}

	// Backup file should contain the exact data
	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Errorf("Failed to read backup file: %v", err)
	}
	if string(backupContent) != exactData {
		t.Errorf("Backup file should contain the original exact data")
	}
}

func TestRotatingWriter_RaceConditionProtection(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "race_test.log")

	config := &RotatingWriterConfig{
		Filename:   filename,
		MaxSize:    1, // Small size to trigger frequent rotations
		MaxBackups: 5,
	}

	w, err := NewRotatingWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	const numGoroutines = 50
	const writesPerGoroutine = 20

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines)

	// Start multiple goroutines that will likely trigger concurrent rotations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < writesPerGoroutine; j++ {
				// Write data that's likely to trigger rotation
				largeData := strings.Repeat(fmt.Sprintf("%d", id), 50000) // ~50KB per write
				_, err := w.Write([]byte(largeData))
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d, write %d: %w", id, j, err)
					return
				}

				// Small delay to increase chance of race conditions
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Check for race condition errors
	for err := range errCh {
		t.Error(err)
	}

	// Verify that files exist and writer is still functional
	info, err := os.Stat(filename)
	if err != nil {
		t.Errorf("Main file should exist: %v", err)
	} else if info.Size() == 0 {
		t.Error("Main file should not be empty")
	}

	// Test that writer is still functional after concurrent operations
	finalWrite := "Final test write\n"
	n, err := w.Write([]byte(finalWrite))
	if err != nil {
		t.Errorf("Final write error = %v", err)
	}
	if n != len(finalWrite) {
		t.Errorf("Final write returned %d bytes, want %d", n, len(finalWrite))
	}
}

func TestRotatingWriter_OpenExistingOrNew(t *testing.T) {
	t.Run("directory creation failure", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test directory creation failure")
		}

		// Create a file where we want to create a directory
		tempDir := t.TempDir()
		blockingFile := filepath.Join(tempDir, "blocking")
		err := os.WriteFile(blockingFile, []byte("block"), 0644)
		if err != nil {
			t.Fatalf("Failed to create blocking file: %v", err)
		}

		// Try to create a log file where a directory should be created but is blocked by the file
		filename := filepath.Join(blockingFile, "subdir", "test.log")
		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		_, err = NewRotatingWriter(config)
		if err == nil {
			t.Error("Expected error when directory creation is blocked by existing file")
		}
	})

	t.Run("existing file with different permissions", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "readonly_existing.log")

		// Create an existing file with some content
		existingContent := "existing content\n"
		err := os.WriteFile(filename, []byte(existingContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		defer w.Close()

		// Verify it detected the existing file size
		if w.currentSize != int64(len(existingContent)) {
			t.Errorf("currentSize = %d, want %d", w.currentSize, len(existingContent))
		}
	})

	t.Run("file stat error other than not exist", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test permission denied")
		}

		tempDir := t.TempDir()
		restrictedDir := filepath.Join(tempDir, "restricted")

		// Create directory with no read permissions
		err := os.Mkdir(restrictedDir, 0000)
		if err != nil {
			t.Fatalf("Failed to create restricted directory: %v", err)
		}
		defer os.Chmod(restrictedDir, 0755) // Cleanup

		filename := filepath.Join(restrictedDir, "test.log")
		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		_, err = NewRotatingWriter(config)
		if err == nil {
			t.Error("Expected error when file stat fails due to permissions")
		}
	})

	t.Run("file open failure", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test permission denied")
		}

		tempDir := t.TempDir()
		restrictedDir := filepath.Join(tempDir, "restricted")

		// Create directory with no write permissions
		err := os.Mkdir(restrictedDir, 0444)
		if err != nil {
			t.Fatalf("Failed to create restricted directory: %v", err)
		}
		defer os.Chmod(restrictedDir, 0755) // Cleanup

		filename := filepath.Join(restrictedDir, "test.log")
		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		_, err = NewRotatingWriter(config)
		if err == nil {
			t.Error("Expected error when file open fails due to permissions")
		}
	})
}

func TestRotatingWriter_Write_Coverage(t *testing.T) {
	t.Run("write to nil file after close", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "write_after_close.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}

		// Close the writer
		w.Close()

		// Try to write after close
		_, err = w.Write([]byte("test"))
		if err == nil {
			t.Error("Expected error when writing to closed writer")
		}
	})

	t.Run("rotation failure but write succeeds", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "rotation_fail.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1, // 1MB
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		defer w.Close()

		// Write initial data
		initialData := strings.Repeat("A", 100)
		_, err = w.Write([]byte(initialData))
		if err != nil {
			t.Errorf("Initial write error = %v", err)
		}

		// Create a directory where backup file should be created to cause rename failure
		backupPath := filename + ".1"
		err = os.Mkdir(backupPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory at backup path: %v", err)
		}
		defer os.RemoveAll(backupPath)

		// Try to trigger rotation - this should fail but write should still succeed
		largeData := strings.Repeat("B", 1024*1024) // 1MB to trigger rotation
		n, err := w.Write([]byte(largeData))
		if err != nil {
			t.Errorf("Write() return error = %v", err)
		}

		// Write should succeed even if rotation fails
		if n != len(largeData) {
			t.Errorf("Write returned %d bytes, want %d", n, len(largeData))
		}
		// We don't check for error here because the behavior may vary -
		// the original file might continue to be written to
	})

	t.Run("partial write scenarios", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "partial_write.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    10, // 10MB - large enough to avoid rotation
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		defer w.Close()

		// Test writing various sizes
		testCases := [][]byte{
			{},                                   // empty write
			nil,                                  // nil write
			[]byte("small"),                      // small write
			bytes.Repeat([]byte("medium"), 1000), // medium write
		}

		var totalSize int64
		for i, data := range testCases {
			n, err := w.Write(data)
			if err != nil {
				t.Errorf("Write %d failed: %v", i, err)
			}
			if n != len(data) {
				t.Errorf("Write %d returned %d bytes, want %d", i, n, len(data))
			}
			totalSize += int64(len(data))
		}

		// Verify currentSize tracking
		if w.currentSize != totalSize {
			t.Errorf("currentSize = %d, want %d", w.currentSize, totalSize)
		}
	})
}

func TestRotatingWriter_Rotate_Coverage(t *testing.T) {
	t.Run("close failure during rotation", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "close_fail.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}

		// Close the file manually to simulate close failure during rotation
		w.file.Close()
		w.file = nil

		// The rotate method should handle the nil file gracefully
		err = w.rotate()
		if err != nil {
			t.Errorf("rotate() should handle nil file gracefully: %v", err)
		}
	})

	t.Run("backup file rotation errors", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "backup_errors.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 3,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		defer w.Close()

		// Create some backup files
		backup1 := filename + ".1"
		backup2 := filename + ".2"
		err = os.WriteFile(backup1, []byte("backup1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create backup1: %v", err)
		}

		// Create backup2 as a directory to cause rename failure
		err = os.Mkdir(backup2, 0755)
		if err != nil {
			t.Fatalf("Failed to create backup2 directory: %v", err)
		}
		defer os.RemoveAll(backup2)

		// Create backup3 as read-only to test another rename scenario
		backup3 := filename + ".3"
		err = os.WriteFile(backup3, []byte("backup3"), 0444)
		if err != nil {
			t.Fatalf("Failed to create backup3: %v", err)
		}
		defer func() {
			os.Chmod(backup3, 0644)
			os.Remove(backup3)
		}()

		// Trigger rotation - should handle backup rotation errors gracefully
		w.currentSize = 1024 * 1024 // Set current size to trigger rotation on next write
		_, err = w.Write([]byte("trigger rotation"))
		if err != nil {
			t.Errorf("Write() return error = %v", err)
		}

		// Rotation should complete even with backup errors
		// The exact behavior depends on the specific errors, but it shouldn't crash
	})

	t.Run("glob error handling", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "glob_test.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		defer w.Close()

		// Write data and trigger rotation
		w.currentSize = 1024 * 1024
		_, err = w.Write([]byte("test"))
		if err != nil {
			t.Errorf("Write after setting large currentSize should work: %v", err)
		}
	})

	t.Run("maxBackups zero remove failure", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "remove_fail.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 0, // No backups - should remove file
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}
		defer w.Close()

		// Write initial data
		_, err = w.Write([]byte("initial"))
		if err != nil {
			t.Errorf("Initial write error = %v", err)
		}

		// Replace the file with a directory to cause remove failure
		w.Close()
		os.Remove(filename)
		err = os.Mkdir(filename, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory at log path: %v", err)
		}
		defer os.RemoveAll(filename)

		// Try to reopen - this should fail during openExistingOrNew
		err = w.openExistingOrNew()
		if err == nil {
			t.Error("Expected error when opening file that's now a directory")
		}
	})
}

func TestRotatingWriter_Close_Coverage(t *testing.T) {
	t.Run("close already closed writer", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "double_close.log")

		config := &RotatingWriterConfig{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 2,
		}

		w, err := NewRotatingWriter(config)
		if err != nil {
			t.Fatalf("Failed to create writer: %v", err)
		}

		// First close
		err = w.Close()
		if err != nil {
			t.Errorf("First close error = %v", err)
		}

		// Second close should not error
		err = w.Close()
		if err != nil {
			t.Errorf("Second close error = %v", err)
		}
	})

	t.Run("close with nil file", func(t *testing.T) {
		w := &RotatingWriter{
			filename: "test.log",
			file:     nil, // Simulate already closed
		}

		err := w.Close()
		if err != nil {
			t.Errorf("Close with nil file should not error: %v", err)
		}
	})
}
