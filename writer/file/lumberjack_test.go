package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/natefinch/lumberjack.v2"
)

func TestNewLumberjackWriter(t *testing.T) {
	tests := []struct {
		name   string
		config *LumberjackConfig
		want   *lumberjack.Logger
	}{
		{
			name: "basic configuration",
			config: &LumberjackConfig{
				Filename:   "test.log",
				MaxSize:    10,
				MaxAge:     7,
				MaxBackups: 3,
				Compress:   true,
			},
			want: &lumberjack.Logger{
				Filename:   "test.log",
				MaxSize:    10,
				MaxAge:     7,
				MaxBackups: 3,
				Compress:   true,
			},
		},
		{
			name: "minimal configuration",
			config: &LumberjackConfig{
				Filename: "minimal.log",
			},
			want: &lumberjack.Logger{
				Filename:   "minimal.log",
				MaxSize:    0,
				MaxAge:     0,
				MaxBackups: 0,
				Compress:   false,
			},
		},
		{
			name: "with subdirectory",
			config: &LumberjackConfig{
				Filename:   "logs/subdir/app.log",
				MaxSize:    100,
				MaxAge:     30,
				MaxBackups: 10,
				Compress:   false,
			},
			want: &lumberjack.Logger{
				Filename:   "logs/subdir/app.log",
				MaxSize:    100,
				MaxAge:     30,
				MaxBackups: 10,
				Compress:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLumberjackWriter(tt.config)
			if err != nil {
				t.Errorf("NewLumberjackWriter() error = %v, want nil", err)
				return
			}

			// Check if the returned logger has the correct configuration
			if got.Filename != tt.want.Filename {
				t.Errorf("NewLumberjackWriter().Filename = %v, want %v", got.Filename, tt.want.Filename)
			}
			if got.MaxSize != tt.want.MaxSize {
				t.Errorf("NewLumberjackWriter().MaxSize = %v, want %v", got.MaxSize, tt.want.MaxSize)
			}
			if got.MaxAge != tt.want.MaxAge {
				t.Errorf("NewLumberjackWriter().MaxAge = %v, want %v", got.MaxAge, tt.want.MaxAge)
			}
			if got.MaxBackups != tt.want.MaxBackups {
				t.Errorf("NewLumberjackWriter().MaxBackups = %v, want %v", got.MaxBackups, tt.want.MaxBackups)
			}
			if got.Compress != tt.want.Compress {
				t.Errorf("NewLumberjackWriter().Compress = %v, want %v", got.Compress, tt.want.Compress)
			}
		})
	}
}

func TestNewLumberjackWriter_NilConfig(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when passing nil config, but no panic occurred")
		}
	}()

	_, _ = NewLumberjackWriter(nil)
}

func TestLumberjackWriter_ImplementsIOWriter(t *testing.T) {
	config := &LumberjackConfig{
		Filename: "test_interface.log",
		MaxSize:  1,
	}

	writer, err := NewLumberjackWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Ensure it implements io.Writer
	var _ io.Writer = writer

	// Test that we can write to it
	testData := "Hello, World!"
	n, err := writer.Write([]byte(testData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(testData))
	}

	// Clean up
	defer os.Remove(config.Filename)
}

func TestLumberjackWriter_FileCreation(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "creation_test.log")

	config := &LumberjackConfig{
		Filename: filename,
		MaxSize:  1,
	}

	writer, err := NewLumberjackWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Write some data
	testData := "Test log entry\n"
	_, err = writer.Write([]byte(testData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Check if file was created and contains the data
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
	}

	if string(content) != testData {
		t.Errorf("File content = %q, want %q", string(content), testData)
	}
}

func TestLumberjackWriter_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir", "nested")
	filename := filepath.Join(subDir, "nested_test.log")

	config := &LumberjackConfig{
		Filename: filename,
		MaxSize:  1,
	}

	writer, err := NewLumberjackWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Write some data
	testData := "Nested directory test\n"
	_, err = writer.Write([]byte(testData))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Check if directory and file were created
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		t.Error("Subdirectory was not created")
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
	}

	if string(content) != testData {
		t.Errorf("File content = %q, want %q", string(content), testData)
	}
}

func TestLumberjackWriter_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "concurrent_test.log")

	config := &LumberjackConfig{
		Filename: filename,
		MaxSize:  10, // 10MB to avoid rotation during test
	}

	writer, err := NewLumberjackWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Test concurrent writes
	const numGoroutines = 10
	const messagesPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < messagesPerGoroutine; j++ {
				message := fmt.Sprintf("Goroutine %d, Message %d\n", id, j)
				_, err := writer.Write([]byte(message))
				if err != nil {
					t.Errorf("Concurrent write error: %v", err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check if file exists and has content
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
	}

	// Count lines to verify all writes succeeded
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	expectedLines := numGoroutines * messagesPerGoroutine

	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, len(lines))
	}
}

func TestLumberjackConfig_ZeroValues(t *testing.T) {
	config := &LumberjackConfig{
		Filename: "zero_values.log",
		// All other fields are zero values
	}

	writer, err := NewLumberjackWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer with zero values: %v", err)
	}

	// Verify zero values are properly set
	if writer.MaxSize != 0 {
		t.Errorf("Expected MaxSize to be 0, got %d", writer.MaxSize)
	}
	if writer.MaxAge != 0 {
		t.Errorf("Expected MaxAge to be 0, got %d", writer.MaxAge)
	}
	if writer.MaxBackups != 0 {
		t.Errorf("Expected MaxBackups to be 0, got %d", writer.MaxBackups)
	}
	if writer.Compress {
		t.Errorf("Expected Compress to be false, got %t", writer.Compress)
	}

	// Test that it still works
	_, err = writer.Write([]byte("Zero values test\n"))
	if err != nil {
		t.Errorf("Write with zero values failed: %v", err)
	}

	// Clean up
	defer os.Remove(config.Filename)
}

func TestLumberjackConfig_LargeValues(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "large_values.log")

	config := &LumberjackConfig{
		Filename:   filename,
		MaxSize:    1000,
		MaxAge:     365,
		MaxBackups: 100,
		Compress:   true,
	}

	writer, err := NewLumberjackWriter(config)
	if err != nil {
		t.Fatalf("Failed to create writer with large values: %v", err)
	}

	// Verify large values are properly set
	if writer.MaxSize != 1000 {
		t.Errorf("Expected MaxSize to be 1000, got %d", writer.MaxSize)
	}
	if writer.MaxAge != 365 {
		t.Errorf("Expected MaxAge to be 365, got %d", writer.MaxAge)
	}
	if writer.MaxBackups != 100 {
		t.Errorf("Expected MaxBackups to be 100, got %d", writer.MaxBackups)
	}
	if !writer.Compress {
		t.Errorf("Expected Compress to be true, got %t", writer.Compress)
	}
}
