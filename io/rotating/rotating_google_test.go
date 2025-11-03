package rotating

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/balinomad/go-mockfs/v2"
)

// TestNewRotatingWriter validates the constructor logic for RotatingWriter.
func TestNewRotatingWriter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      *RotatingWriterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_config",
			config: &RotatingWriterConfig{
				Filename:   "valid.log",
				MaxSize:    10,
				MaxBackups: 3,
			},
		},
		{
			name: "valid_config_with_subdirectory",
			config: &RotatingWriterConfig{
				Filename:   filepath.Join("logs", "app.log"),
				MaxSize:    100,
				MaxBackups: 5,
			},
		},
		{
			name: "valid_config_with_zero_values",
			config: &RotatingWriterConfig{
				Filename:   "zero.log",
				MaxSize:    0,
				MaxBackups: 0,
			},
		},
		{
			name:        "invalid_config_empty_filename",
			config:      &RotatingWriterConfig{Filename: ""},
			expectError: true,
			errorMsg:    "filename cannot be empty",
		},
		{
			name:        "invalid_config_negative_max_size",
			config:      &RotatingWriterConfig{Filename: "test.log", MaxSize: -1},
			expectError: true,
			errorMsg:    "max size must be non-negative",
		},
		{
			name:        "invalid_config_negative_max_backups",
			config:      &RotatingWriterConfig{Filename: "test.log", MaxBackups: -1},
			expectError: true,
			errorMsg:    "max backups must be non-negative",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()
			if !tc.expectError {
				tc.config.Filename = filepath.Join(tempDir, tc.config.Filename)
			}

			w, err := NewRotatingWriter(tc.config)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected an error but got none")
				}
				if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("error message %q does not contain %q", err.Error(), tc.errorMsg)
				}
				if w != nil {
					t.Error("writer should be nil on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if w == nil {
					t.Fatal("writer should not be nil")
				}
				defer w.Close()

				if w.filename != tc.config.Filename {
					t.Errorf("filename: got %q, want %q", w.filename, tc.config.Filename)
				}
				expectedMaxSize := int64(tc.config.MaxSize) * 1024 * 1024
				if w.maxSize != expectedMaxSize {
					t.Errorf("maxSize: got %d, want %d", w.maxSize, expectedMaxSize)
				}
				if w.maxBackups != tc.config.MaxBackups {
					t.Errorf("maxBackups: got %d, want %d", w.maxBackups, tc.config.MaxBackups)
				}
				if w.file == nil {
					t.Error("internal file handle should not be nil")
				}
			}
		})
	}
}

// TestRotatingWriter_WriteAndRotate is a comprehensive table-driven test covering
// standard writes, writes to existing files, and various rotation scenarios.
func TestRotatingWriter_WriteAndRotate(t *testing.T) {
	t.Parallel()

	over1MB := strings.Repeat("a", 1024*1024+1)

	testCases := []struct {
		name            string
		config          *RotatingWriterConfig
		initialContent  string
		writes          [][]byte
		expectedFile    string
		expectedBackups map[string]string // map[backupFilename]content
	}{
		{
			name:         "single_small_write_no_rotation",
			config:       &RotatingWriterConfig{MaxSize: 1, MaxBackups: 3},
			writes:       [][]byte{[]byte("hello world")},
			expectedFile: "hello world",
		},
		{
			name:           "write_to_existing_file_appends",
			config:         &RotatingWriterConfig{MaxSize: 1, MaxBackups: 3},
			initialContent: "existing data. ",
			writes:         [][]byte{[]byte("new data.")},
			expectedFile:   "existing data. new data.",
		},
		{
			name:         "simple_rotation",
			config:       &RotatingWriterConfig{MaxSize: 1, MaxBackups: 3},
			writes:       [][]byte{[]byte("initial write. "), []byte(over1MB)},
			expectedFile: over1MB,
			expectedBackups: map[string]string{
				"test.log.1": "initial write. ",
			},
		},
		{
			name:   "multiple_rotations_purge_oldest",
			config: &RotatingWriterConfig{MaxSize: 1, MaxBackups: 2},
			writes: [][]byte{
				[]byte("content1"), []byte(over1MB), // Rotates, content1 -> .1
				[]byte("content2"), []byte(over1MB), // Rotates, content2 -> .1, .1 -> .2
				[]byte("content3"), []byte(over1MB), // Rotates, content3 -> .1, .1 -> .2, old .2 purged
			},
			expectedFile: over1MB,
			expectedBackups: map[string]string{
				"test.log.1": "content3",
				"test.log.2": "content2",
			},
		},
		{
			name:           "rotation_with_zero_max_backups_removes_original",
			config:         &RotatingWriterConfig{MaxSize: 1, MaxBackups: 0},
			initialContent: "this will be removed",
			writes:         [][]byte{[]byte(over1MB)},
			expectedFile:   over1MB,
		},
		{
			name:         "no_rotation_with_zero_max_size",
			config:       &RotatingWriterConfig{MaxSize: 0, MaxBackups: 3},
			writes:       [][]byte{[]byte("content1"), []byte(over1MB)},
			expectedFile: "content1" + over1MB,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "test.log")
			tc.config.Filename = filename

			if tc.initialContent != "" {
				if err := os.WriteFile(filename, []byte(tc.initialContent), 0644); err != nil {
					t.Fatalf("failed to write initial file: %v", err)
				}
			}

			w, err := NewRotatingWriter(tc.config)
			if err != nil {
				t.Fatalf("NewRotatingWriter() failed: %v", err)
			}

			for i, data := range tc.writes {
				if _, err := w.Write(data); err != nil {
					t.Errorf("Write %d failed: %v", i, err)
				}
			}
			if err := w.Close(); err != nil {
				t.Errorf("Close() failed: %v", err)
			}

			assertFileContent(t, filename, tc.expectedFile)

			for backupFile, expectedContent := range tc.expectedBackups {
				assertFileContent(t, filepath.Join(tempDir, backupFile), expectedContent)
			}

			purgedBackup := fmt.Sprintf("%s.%d", filename, tc.config.MaxBackups+1)
			assertFileNotExists(t, purgedBackup)

			if tc.config.MaxBackups == 0 && len(tc.writes) > 0 {
				assertFileNotExists(t, filename+".1")
			}
		})
	}
}

// TestRotatingWriter_Close validates the Close method behavior.
func TestRotatingWriter_Close(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "close.log")

	w, err := NewRotatingWriter(&RotatingWriterConfig{Filename: filename, MaxSize: 1, MaxBackups: 1})
	if err != nil {
		t.Fatalf("NewRotatingWriter() failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Errorf("first Close() failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close() should not fail: %v", err)
	}

	_, err = w.Write([]byte("write after close"))
	if err == nil {
		t.Error("expected error when writing to a closed writer, but got nil")
	}
}

// TestRotatingWriter_Concurrency tests safe concurrent writes, including during rotation.
func TestRotatingWriter_Concurrency(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "concurrent.log")

	w, err := NewRotatingWriter(&RotatingWriterConfig{Filename: filename, MaxSize: 1, MaxBackups: 5})
	if err != nil {
		t.Fatalf("NewRotatingWriter() failed: %v", err)
	}
	defer w.Close()

	const numGoroutines = 20
	const writesPerGoroutine = 10
	writeData := []byte(strings.Repeat("x", 50*1024))
	totalDataSize := int64(numGoroutines * writesPerGoroutine * len(writeData))

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				if _, err := w.Write(writeData); err != nil {
					t.Errorf("write in goroutine failed: %v", err)
				}
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()
	if err := w.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	files, err := filepath.Glob(filename + "*")
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("log files should have been created")
	}

	var actualTotalSize int64
	for _, file := range files {
		info, statErr := os.Stat(file)
		if statErr != nil {
			t.Fatalf("Stat failed for %s: %v", file, statErr)
		}
		actualTotalSize += info.Size()
	}

	if totalDataSize != actualTotalSize {
		t.Errorf("total size mismatch: got %d, want %d", actualTotalSize, totalDataSize)
	}
}

// TestRotatingWriter_FilesystemErrors uses a mock to test unrecoverable filesystem errors.
func TestRotatingWriter_FilesystemErrors(t *testing.T) {
	t.Parallel()
	mockErr := errors.New("mock filesystem error")
	logFileName := "mocktest.log"

	testCases := []struct {
		name          string
		setupMock     func(mfs *mockfs.MockFS, filename string)
		action        func(filename string) (*RotatingWriter, error) // action returns the error from NewRotatingWriter or a Write call
		expectedError string
	}{
		// {
		// 	name: "initial_mkdir_fails",
		// 	setupMock: func(mfs *mockfs.MockFS, filename string) {
		// 		mfs.AddMkdirallError(filepath.Dir(filename), mockErr)
		// 	},
		// 	action: func(fn string) (*RotatingWriter, error) {
		// 		return NewRotatingWriter(&RotatingWriterConfig{Filename: fn})
		// 	},
		// 	expectedError: "failed to create log directory",
		// },
		{
			name: "initial_stat_fails",
			setupMock: func(mfs *mockfs.MockFS, filename string) {
				mfs.FailStat(filename, mockErr)
			},
			action: func(fn string) (*RotatingWriter, error) {
				return NewRotatingWriter(&RotatingWriterConfig{Filename: fn})
			},
			expectedError: "failed to get file info",
		},
		{
			name: "initial_open_fails",
			setupMock: func(mfs *mockfs.MockFS, filename string) {
				mfs.FailOpen(filename, mockErr)
			},
			action: func(fn string) (*RotatingWriter, error) {
				return NewRotatingWriter(&RotatingWriterConfig{Filename: fn})
			},
			expectedError: "failed to open log file",
		},
		{
			name: "direct_write_fails",
			setupMock: func(mfs *mockfs.MockFS, filename string) {
				mfs.FailWrite(filename, mockErr)
			},
			action: func(fn string) (*RotatingWriter, error) {
				w, err := NewRotatingWriter(&RotatingWriterConfig{Filename: fn})
				if err != nil {
					return nil, fmt.Errorf("setup failed: %w", err)
				}
				_, writeErr := w.Write([]byte("test"))
				return w, writeErr
			},
			expectedError: "mock filesystem error",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mfs := mockfs.NewMockFS(nil)
			mfs.AddFile(logFileName, "", 0644)
			tc.setupMock(mfs, logFileName)

			w, err := tc.action(logFileName)
			if w != nil {
				defer w.Close()
			}

			if err == nil {
				t.Fatalf("expected error containing %q but got none", tc.expectedError)
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.expectedError)
			}
		})
	}
}

// TestRotatingWriter_RotationFailureRecovery tests that a write succeeds even if the internal rotation fails.
/*
func TestRotatingWriter_RotationFailureRecovery(t *testing.T) {
	t.Parallel()
	mockErr := errors.New("mock rename/remove error")

	testCases := []struct {
		name       string
		maxBackups int
		setupMock  func(mfs *mockfs.MockFS, filename string)
	}{
		{
			name:       "rename_fails_during_rotation",
			maxBackups: 1,
			setupMock: func(mfs *mockfs.MockFS, filename string) {
				mfs.AddRenameError(filename, filename+".1", mockErr)
			},
		},
		{
			name:       "remove_fails_with_zero_backups",
			maxBackups: 0,
			setupMock: func(mfs *mockfs.MockFS, filename string) {
				mfs.AddRemoveError(filename, mockErr)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mfs := mockfs.New()
			mfs.Patch()
			defer mfs.Unpatch()
			tc.setupMock(mfs)

			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "recovery.log")

			tc.setupMock(mfs, filename)

			w, err := NewRotatingWriter(&RotatingWriterConfig{
				Filename:   filename,
				MaxSize:    1,
				MaxBackups: tc.maxBackups,
			})
			if err != nil {
				t.Fatalf("NewRotatingWriter failed: %v", err)
			}

			initialData := "initial data. "
			if _, err := w.Write([]byte(initialData)); err != nil {
				t.Fatalf("Initial write failed: %v", err)
			}

			// This write triggers rotation. The rotation should fail internally due to the mock,
			// but the writer should recover and the write should succeed by appending to the original file.
			rotatingData := strings.Repeat("a", 1024*1024)
			if _, err := w.Write([]byte(rotatingData)); err != nil {
				t.Fatalf("Rotating write should have succeeded but failed: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			// Verify that rotation did not happen and data was preserved.
			assertFileNotExists(t, filename+".1")
			assertFileContent(t, filename, initialData+rotatingData)
		})
	}
}*/

// assertFileContent is a test helper to check if a file's content matches the expected string.
func assertFileContent(t *testing.T, filename, expected string) {
	t.Helper()
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", filename, err)
	}
	if expected != string(content) {
		t.Errorf("file content mismatch for %s:\ngot:  %q\nwant: %q", filename, string(content), expected)
	}
}

// assertFileNotExists is a test helper to check that a file does not exist.
func assertFileNotExists(t *testing.T, filename string) {
	t.Helper()
	_, err := os.Stat(filename)
	if !os.IsNotExist(err) {
		if err == nil {
			t.Errorf("file should not exist but it does: %s", filename)
		} else {
			t.Errorf("expected a file-not-exist error for %s, but got: %v", filename, err)
		}
	}
}
