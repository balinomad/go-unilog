package handler_test

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

// TestNewBaseHandler verifies the constructor for BaseHandler.
func TestNewBaseHandler(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		opts := handler.BaseOptions{
			Level:  handler.WarnLevel,
			Output: io.Discard,
		}
		h, err := handler.NewBaseHandler(opts)
		if err != nil {
			t.Fatalf("NewBaseHandler() error = %v, want nil", err)
		}
		if h == nil {
			t.Fatal("NewBaseHandler() handler = nil, want non-nil")
		}
	})

	t.Run("nil_writer_error", func(t *testing.T) {
		t.Parallel()
		opts := handler.BaseOptions{
			Level:  handler.InfoLevel,
			Output: nil, // This should cause atomicwriter.NewAtomicWriter to fail
		}
		h, err := handler.NewBaseHandler(opts)
		if err == nil {
			t.Error("NewBaseHandler() error = nil, want error")
		}
		if h != nil {
			t.Errorf("NewBaseHandler() handler = %v, want nil", h)
		}
		// The error check in NewBaseHandler wraps the underlying error.
		if !errors.Is(err, handler.ErrAtomicWriterFail) {
			t.Errorf("NewBaseHandler() error = %v, want wrapped ErrAtomicWriterFail", err)
		}
	})
}

// TestBaseHandler_Enabled verifies the Enabled method logic.
func TestBaseHandler_Enabled(t *testing.T) {
	t.Parallel()
	opts := handler.BaseOptions{
		Level:  handler.InfoLevel,
		Output: io.Discard,
	}
	h, err := handler.NewBaseHandler(opts)
	if err != nil {
		t.Fatalf("NewBaseHandler() failed: %v", err)
	}

	tests := []struct {
		name  string
		level handler.LogLevel
		want  bool
	}{
		{"level_below", handler.DebugLevel, false},
		{"level_equal", handler.InfoLevel, true},
		{"level_above", handler.WarnLevel, true},
		{"level_max", handler.MaxLevel, true},
		{"level_min", handler.MinLevel, false},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := h.Enabled(tt.level); got != tt.want {
				t.Errorf("Enabled(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

// TestBaseHandler_SetLevel verifies the SetLevel method.
func TestBaseHandler_SetLevel(t *testing.T) {
	t.Parallel()
	opts := handler.BaseOptions{
		Level:  handler.InfoLevel,
		Output: io.Discard,
	}
	// Each subtest creates its own isolated handler instance to ensure test isolation and parallel safety.

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		h, err := handler.NewBaseHandler(opts)
		if err != nil {
			t.Fatalf("NewBaseHandler() failed: %v", err)
		}

		newLevel := handler.WarnLevel
		if err := h.SetLevel(newLevel); err != nil {
			t.Fatalf("SetLevel(%v) error = %v, want nil", newLevel, err)
		}

		// Verify Enabled reflects the change
		if !h.Enabled(handler.WarnLevel) {
			t.Error("Enabled(WarnLevel) = false, want true after SetLevel")
		}
		if h.Enabled(handler.InfoLevel) {
			t.Error("Enabled(InfoLevel) = true, want false after SetLevel")
		}
	})

	t.Run("invalid_level_low", func(t *testing.T) {
		t.Parallel()
		h, err := handler.NewBaseHandler(opts) // New handler for isolation
		if err != nil {
			t.Fatalf("NewBaseHandler() failed: %v", err)
		}

		invalidLevel := handler.MinLevel - 1
		err = h.SetLevel(invalidLevel)
		if err == nil {
			t.Errorf("SetLevel(%v) error = nil, want error", invalidLevel)
		}
		if !errors.Is(err, handler.ErrInvalidLogLevel) {
			t.Errorf("SetLevel() error = %v, want ErrInvalidLogLevel", err)
		}
		// Ensure level was not changed
		if !h.Enabled(handler.InfoLevel) {
			t.Error("Enabled(InfoLevel) = false, want true (level should not have changed)")
		}
		if h.Enabled(handler.TraceLevel) {
			t.Error("Enabled(TraceLevel) = true, want false (level should not have changed)")
		}
	})

	t.Run("invalid_level_high", func(t *testing.T) {
		t.Parallel()
		h, err := handler.NewBaseHandler(opts) // New handler for isolation
		if err != nil {
			t.Fatalf("NewBaseHandler() failed: %v", err)
		}

		invalidLevel := handler.MaxLevel + 1
		err = h.SetLevel(invalidLevel)
		if err == nil {
			t.Errorf("SetLevel(%v) error = nil, want error", invalidLevel)
		}
		if !errors.Is(err, handler.ErrInvalidLogLevel) {
			t.Errorf("SetLevel() error = %v, want ErrInvalidLogLevel", err)
		}
		// Ensure level was not changed (still enabled at original InfoLevel)
		if !h.Enabled(handler.InfoLevel) {
			t.Error("Enabled(InfoLevel) = false, want true (level should not have changed)")
		}
		if h.Enabled(handler.TraceLevel) {
			t.Error("Enabled(TraceLevel) = true, want false (level should not have changed)")
		}
	})

	t.Run("concurrent_setlevel", func(t *testing.T) {
		t.Parallel()
		h, err := handler.NewBaseHandler(opts)
		if err != nil {
			t.Fatalf("NewBaseHandler() failed: %v", err)
		}

		var wg sync.WaitGroup
		levels := []handler.LogLevel{
			handler.TraceLevel,
			handler.DebugLevel,
			handler.InfoLevel,
			handler.WarnLevel,
			handler.ErrorLevel,
		}

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(level handler.LogLevel) {
				defer wg.Done()
				_ = h.SetLevel(level)
			}(levels[i%len(levels)])
		}
		wg.Wait()

		// After concurrent writes, the handler must remain in a valid, functional state.
		// If the atomic value was corrupted, Enabled might return an inconsistent result or panic.
		if !h.Enabled(handler.MaxLevel) {
			t.Error("Enabled(MaxLevel) = false, want true (handler must be functional)")
		}
		if h.Enabled(handler.MinLevel - 1) {
			t.Error("Enabled(MinLevel-1) = true, want false (level should be within valid range)")
		}
	})
}

// TestBaseHandler_SetOutput verifies the SetOutput method.
func TestBaseHandler_SetOutput(t *testing.T) {
	t.Parallel()
	var buf1 bytes.Buffer
	opts := handler.BaseOptions{
		Level:  handler.InfoLevel,
		Output: &buf1,
	}
	h, err := handler.NewBaseHandler(opts)
	if err != nil {
		t.Fatalf("NewBaseHandler() failed: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var buf2 bytes.Buffer
		if err := h.SetOutput(&buf2); err != nil {
			t.Fatalf("SetOutput() error = %v, want nil", err)
		}

		// Verify new writer is used (by writing to the underlying atomic writer)
		testStr := "hello"
		aw := h.AtomicWriter()
		if _, err := aw.Write([]byte(testStr)); err != nil {
			t.Fatalf("AtomicWriter.Write() failed: %v", err)
		}

		if buf1.Len() > 0 {
			t.Errorf("original writer buf.Len() = %d, want 0", buf1.Len())
		}
		if buf2.String() != testStr {
			t.Errorf("new writer buf.String() = %q, want %q", buf2.String(), testStr)
		}
	})

	t.Run("nil_writer_error", func(t *testing.T) {
		t.Parallel()
		// Test on a fresh handler instance to avoid side effects
		h, err := handler.NewBaseHandler(opts)
		if err != nil {
			t.Fatalf("NewBaseHandler() failed: %v", err)
		}

		err = h.SetOutput(nil)
		if err == nil {
			t.Error("SetOutput(nil) error = nil, want error")
		}
		// The error should contain the sentinel error ErrNilWriter defined in the handler package.
		if !errors.Is(err, handler.ErrNilWriter) {
			t.Errorf("SetOutput(nil) error = %v, want error containing %v", err, handler.ErrNilWriter)
		}
	})
}

// TestBaseHandler_AtomicWriter verifies the AtomicWriter getter.
func TestBaseHandler_AtomicWriter(t *testing.T) {
	t.Parallel()
	opts := handler.BaseOptions{
		Level:  handler.InfoLevel,
		Output: io.Discard,
	}
	h, err := handler.NewBaseHandler(opts)
	if err != nil {
		t.Fatalf("NewBaseHandler() failed: %v", err)
	}

	aw := h.AtomicWriter()
	if aw == nil {
		t.Fatal("AtomicWriter() = nil, want non-nil")
	}
}
