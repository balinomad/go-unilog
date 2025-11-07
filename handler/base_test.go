package handler_test

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

// Helpers

func newHandler(t *testing.T, opts handler.BaseOptions) *handler.BaseHandler {
	t.Helper()
	h, err := handler.NewBaseHandler(opts)
	if err != nil {
		t.Fatalf("NewBaseHandler() failed: %v", err)
	}
	return h
}

func assertEnabled(t *testing.T, h *handler.BaseHandler, lvl handler.LogLevel, want bool) {
	t.Helper()
	if got := h.Enabled(lvl); got != want {
		t.Fatalf("Enabled(%v) = %v, want %v", lvl, got, want)
	}
}

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
			Output: nil,
		}
		h, err := handler.NewBaseHandler(opts)
		if err == nil {
			t.Error("NewBaseHandler() error = nil, want error")
		}
		if h != nil {
			t.Errorf("NewBaseHandler() handler = %v, want nil", h)
		}
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
	h := newHandler(t, opts)

	tests := []struct {
		name  string
		level handler.LogLevel
		want  bool
	}{
		{"below configured level", handler.DebugLevel, false},
		{"at configured level", handler.InfoLevel, true},
		{"above configured level", handler.WarnLevel, true},
		{"max level", handler.MaxLevel, true},
		{"min level", handler.MinLevel, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := h.Enabled(tt.level); got != tt.want {
				t.Errorf("Enabled(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

// TestBaseHandler_SetLevel_change verifies that SetLevel updates the handler level.
func TestBaseHandler_SetLevel_change(t *testing.T) {
	t.Parallel()
	opts := handler.BaseOptions{Level: handler.InfoLevel, Output: io.Discard}
	h := newHandler(t, opts)

	t.Run("change level succeeds", func(t *testing.T) {
		t.Parallel()
		newLevel := handler.WarnLevel
		if err := h.SetLevel(newLevel); err != nil {
			t.Fatalf("SetLevel(%v) error = %v, want nil", newLevel, err)
		}
		assertEnabled(t, h, handler.WarnLevel, true)
		assertEnabled(t, h, handler.InfoLevel, false)
	})
}

// TestBaseHandler_SetLevel_invalid verifies SetLevel rejects out-of-range values.
// Uses table-driven tests for both below-minimum and above-maximum cases.
func TestBaseHandler_SetLevel_invalid(t *testing.T) {
	t.Parallel()
	opts := handler.BaseOptions{Level: handler.InfoLevel, Output: io.Discard}

	cases := []struct {
		name        string
		invalid     handler.LogLevel
		description string
	}{
		{"rejects level below minimum", handler.MinLevel - 1, "below min"},
		{"rejects level above maximum", handler.MaxLevel + 1, "above max"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHandler(t, opts)
			err := h.SetLevel(tc.invalid)
			if err == nil {
				t.Fatalf("SetLevel(%v) error = nil, want error", tc.invalid)
			}
			if !errors.Is(err, handler.ErrInvalidLogLevel) {
				t.Fatalf("SetLevel() error = %v, want ErrInvalidLogLevel", err)
			}
			// Ensure original level remains in effect.
			assertEnabled(t, h, handler.InfoLevel, true)
			assertEnabled(t, h, handler.TraceLevel, false)
		})
	}
}

// TestBaseHandler_SetLevel_concurrent verifies concurrent SetLevel calls do not corrupt state.
func TestBaseHandler_SetLevel_concurrent(t *testing.T) {
	t.Parallel()
	opts := handler.BaseOptions{Level: handler.InfoLevel, Output: io.Discard}
	h := newHandler(t, opts)

	t.Run("concurrent calls remain safe", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		levels := []handler.LogLevel{
			handler.TraceLevel,
			handler.DebugLevel,
			handler.InfoLevel,
			handler.WarnLevel,
			handler.ErrorLevel,
		}

		const goroutines = 100
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			i := i
			go func() {
				defer wg.Done()
				_ = h.SetLevel(levels[i%len(levels)])
			}()
		}
		wg.Wait()

		// After concurrent writes, handler must remain functional and within valid range.
		assertEnabled(t, h, handler.MaxLevel, true)
		if h.Enabled(handler.MinLevel - 1) {
			t.Fatal("Enabled(MinLevel-1) = true, want false (level should be within valid range)")
		}
	})
}

// errWriter wraps an io.Writer and fails on Sync with a specific error to simulate swap failures.
type errWriter struct {
	err error
}

func (w *errWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (w *errWriter) Sync() error {
	return w.err
}

// TestBaseHandler_SetOutput verifies the SetOutput method.
func TestBaseHandler_SetOutput(t *testing.T) {
	t.Parallel()
	var buf1 bytes.Buffer
	opts := handler.BaseOptions{
		Level:  handler.InfoLevel,
		Output: &buf1,
	}

	t.Run("swaps output writer", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, opts)
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

	t.Run("rejects nil writer", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, opts)
		err := h.SetOutput(nil)
		if err == nil {
			t.Error("SetOutput(nil) error = nil, want error")
		}
		// The error should contain the sentinel error ErrNilWriter defined in the handler package.
		if !errors.Is(err, handler.ErrNilWriter) {
			t.Errorf("SetOutput(nil) error = %v, want error containing %v", err, handler.ErrNilWriter)
		}
	})

	t.Run("wraps non-nil swap errors", func(t *testing.T) {
		t.Parallel()

		customErr := errors.New("disk full")
		badWriter := &errWriter{err: customErr}
		h := newHandler(t, handler.BaseOptions{Level: handler.InfoLevel, Output: badWriter})

		err := h.SetOutput(&buf1)
		if err == nil {
			t.Fatal("SetOutput(errWriter) error = nil, want error")
		}
		if !errors.Is(err, handler.ErrAtomicWriterFail) {
			t.Fatalf("SetOutput() error = %v, want wrapped ErrAtomicWriterFail", err)
		}
		if !errors.Is(err, customErr) {
			t.Fatalf("SetOutput() error = %v, want wrapped customErr", err)
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
	h := newHandler(t, opts)

	aw := h.AtomicWriter()
	if aw == nil {
		t.Fatal("AtomicWriter() = nil, want non-nil")
	}
}
