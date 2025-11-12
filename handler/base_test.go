package handler_test

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

// --- Helpers ---

// newHandler is a test helper to create a BaseHandler, failing the test on error.
func newHandler(t *testing.T, opts *handler.BaseOptions) *handler.BaseHandler {
	t.Helper()
	h, err := handler.NewBaseHandler(opts)
	if err != nil {
		t.Fatalf("NewBaseHandler() failed: %v", err)
	}
	return h
}

// TestNewBaseHandler verifies the constructor and option validation.
func TestNewBaseHandler(t *testing.T) {
	t.Parallel()

	validFormats := []string{"json", "text"}

	tests := []struct {
		name     string
		opts     *handler.BaseOptions
		wantOpts handler.BaseOptions // Expected values after initialization
		wantErr  error
	}{
		{
			name: "success defaults",
			opts: &handler.BaseOptions{
				Output: io.Discard,
			},
			wantOpts: handler.BaseOptions{
				Level:     handler.LogLevel(0),
				Output:    io.Discard,
				Separator: handler.DefaultKeySeparator,
			},
			wantErr: nil,
		},
		{
			name: "success custom",
			opts: &handler.BaseOptions{
				Level:      handler.WarnLevel,
				Output:     io.Discard,
				Format:     "text",
				WithCaller: true,
				WithTrace:  true,
				CallerSkip: 2,
				Separator:  "::",
			},
			wantOpts: handler.BaseOptions{
				Level:      handler.WarnLevel,
				Output:     io.Discard,
				Format:     "text",
				WithCaller: true,
				WithTrace:  true,
				CallerSkip: 2,
				Separator:  "::",
			},
			wantErr: nil,
		},
		{
			name: "error nil writer",
			opts: &handler.BaseOptions{
				Output: nil,
			},
			wantErr: handler.ErrAtomicWriterFail,
		},
		{
			name: "success valid formats default",
			opts: &handler.BaseOptions{
				Output:       io.Discard,
				ValidFormats: validFormats,
				Format:       "", // Should default to "json"
			},
			wantOpts: handler.BaseOptions{
				Output:       io.Discard,
				ValidFormats: validFormats,
				Format:       "json",
				Separator:    handler.DefaultKeySeparator,
			},
			wantErr: nil,
		},
		{
			name: "success valid formats explicit",
			opts: &handler.BaseOptions{
				Output:       io.Discard,
				ValidFormats: validFormats,
				Format:       "text",
			},
			wantOpts: handler.BaseOptions{
				Output:       io.Discard,
				ValidFormats: validFormats,
				Format:       "text",
				Separator:    handler.DefaultKeySeparator,
			},
			wantErr: nil,
		},
		{
			name: "error invalid format",
			opts: &handler.BaseOptions{
				Output:       io.Discard,
				ValidFormats: validFormats,
				Format:       "xml",
			},
			wantErr: handler.ErrInvalidFormat,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, err := handler.NewBaseHandler(tc.opts)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatal("NewBaseHandler() error = nil, want non-nil error")
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewBaseHandler() error = %v, want error chain containing %v", err, tc.wantErr)
				}
				if h != nil {
					t.Fatalf("NewBaseHandler() handler = %v, want nil", h)
				}
				return // Test finished
			}

			if err != nil {
				t.Fatalf("NewBaseHandler() error = %v, want nil", err)
			}
			if h == nil {
				t.Fatal("NewBaseHandler() handler = nil, want non-nil")
			}

			// Verify initial state
			if got := h.Level(); got != tc.wantOpts.Level {
				t.Errorf("Level() = %v, want %v", got, tc.wantOpts.Level)
			}
			if got := h.Format(); got != tc.wantOpts.Format {
				t.Errorf("Format() = %q, want %q", got, tc.wantOpts.Format)
			}
			if got := h.Separator(); got != tc.wantOpts.Separator {
				t.Errorf("Separator() = %q, want %q", got, tc.wantOpts.Separator)
			}
			if got := h.CallerEnabled(); got != tc.wantOpts.WithCaller {
				t.Errorf("CallerEnabled() = %v, want %v", got, tc.wantOpts.WithCaller)
			}
			if got := h.TraceEnabled(); got != tc.wantOpts.WithTrace {
				t.Errorf("TraceEnabled() = %v, want %v", got, tc.wantOpts.WithTrace)
			}
			if got := h.CallerSkip(); got != tc.wantOpts.CallerSkip {
				t.Errorf("CallerSkip() = %v, want %v", got, tc.wantOpts.CallerSkip)
			}
			if h.AtomicWriter() == nil {
				t.Error("AtomicWriter() = nil, want non-nil")
			}
		})
	}
}

// --- Test BaseOptions ---

func TestBaseOption_WithLevel(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{}
		err := handler.WithLevel(handler.WarnLevel)(opts)
		if err != nil {
			t.Fatalf("WithLevel() error = %v, want nil", err)
		}
		if opts.Level != handler.WarnLevel {
			t.Errorf("Level = %v, want %v", opts.Level, handler.WarnLevel)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{}
		err := handler.WithLevel(handler.MinLevel - 1)(opts)
		if err == nil {
			t.Fatal("WithLevel() error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrOptionApplyFailed) {
			t.Fatalf("WithLevel() error = %v, want %v", err, handler.ErrOptionApplyFailed)
		}
	})
}

func TestBaseOption_WithOutput(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{}
		err := handler.WithOutput(io.Discard)(opts)
		if err != nil {
			t.Fatalf("WithOutput() error = %v, want nil", err)
		}
		if opts.Output != io.Discard {
			t.Errorf("Output = %v, want %v", opts.Output, io.Discard)
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{}
		err := handler.WithOutput(nil)(opts)
		if err == nil {
			t.Fatal("WithOutput(nil) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrOptionApplyFailed) {
			t.Fatalf("WithOutput(nil) error = %v, want %v", err, handler.ErrOptionApplyFailed)
		}
	})
}

func TestBaseOption_WithFormat(t *testing.T) {
	t.Parallel()
	opts := &handler.BaseOptions{}
	err := handler.WithFormat("json")(opts)
	if err != nil {
		t.Fatalf("WithFormat() error = %v, want nil", err)
	}
	if opts.Format != "json" {
		t.Errorf("Format = %q, want %q", opts.Format, "json")
	}
}

func TestBaseOption_WithSeparator(t *testing.T) {
	t.Parallel()
	opts := &handler.BaseOptions{}
	err := handler.WithSeparator("::")(opts)
	if err != nil {
		t.Fatalf("WithSeparator() error = %v, want nil", err)
	}
	if opts.Separator != "::" {
		t.Errorf("Separator = %q, want %q", opts.Separator, "::")
	}
}

func TestBaseOption_WithCaller(t *testing.T) {
	t.Parallel()
	t.Run("true", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{}
		err := handler.WithCaller(true)(opts)
		if err != nil {
			t.Fatalf("WithCaller(true) error = %v, want nil", err)
		}
		if !opts.WithCaller {
			t.Error("WithCaller = false, want true")
		}
	})
	t.Run("false", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{WithCaller: true}
		err := handler.WithCaller(false)(opts)
		if err != nil {
			t.Fatalf("WithCaller(false) error = %v, want nil", err)
		}
		if opts.WithCaller {
			t.Error("WithCaller = true, want false")
		}
	})
}

func TestBaseOption_WithTrace(t *testing.T) {
	t.Parallel()
	t.Run("true", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{}
		err := handler.WithTrace(true)(opts)
		if err != nil {
			t.Fatalf("WithTrace(true) error = %v, want nil", err)
		}
		if !opts.WithTrace {
			t.Error("WithTrace = false, want true")
		}
	})
	t.Run("false", func(t *testing.T) {
		t.Parallel()
		opts := &handler.BaseOptions{WithTrace: true}
		err := handler.WithTrace(false)(opts)
		if err != nil {
			t.Fatalf("WithTrace(false) error = %v, want nil", err)
		}
		if opts.WithTrace {
			t.Error("WithTrace = true, want false")
		}
	})
}

// --- Test State Accessors ---

// TestBaseHandler_StateAccessors verifies getters (Level, Format, etc.).
func TestBaseHandler_StateAccessors(t *testing.T) {
	t.Parallel()
	opts := &handler.BaseOptions{
		Level:      handler.WarnLevel,
		Output:     io.Discard,
		Format:     "text",
		WithCaller: true,
		WithTrace:  false,
		CallerSkip: 2,
		Separator:  "::",
	}
	h := newHandler(t, opts)
	h = h.WithKeyPrefix("group1") // Set prefix for KeyPrefix test

	if got := h.Level(); got != handler.WarnLevel {
		t.Errorf("Level() = %v, want %v", got, handler.WarnLevel)
	}
	if got := h.Format(); got != "text" {
		t.Errorf("Format() = %q, want %q", got, "text")
	}
	if got := h.CallerEnabled(); got != true {
		t.Errorf("CallerEnabled() = %v, want %v", got, true)
	}
	if got := h.TraceEnabled(); got != false {
		t.Errorf("TraceEnabled() = %v, want %v", got, false)
	}
	if got := h.CallerSkip(); got != 2 {
		t.Errorf("CallerSkip() = %v, want %v", got, 2)
	}
	if got := h.Separator(); got != "::" {
		t.Errorf("Separator() = %q, want %q", got, "::")
	}
	if got := h.KeyPrefix(); got != "group1" {
		t.Errorf("KeyPrefix() = %q, want %q", got, "group1")
	}
	if h.AtomicWriter() == nil {
		t.Error("AtomicWriter() = nil, want non-nil")
	}
	if h.HandlerState() == nil {
		t.Error("HandlerState() = nil, want non-nil")
	}

	// Test HandlerState interface compliance
	var hs handler.HandlerState = h.HandlerState()
	if got := hs.CallerEnabled(); got != true {
		t.Errorf("HandlerState.CallerEnabled() = %v, want %v", got, true)
	}
	if got := hs.TraceEnabled(); got != false {
		t.Errorf("HandlerState.TraceEnabled() = %v, want %v", got, false)
	}
	if got := hs.CallerSkip(); got != 2 {
		t.Errorf("HandlerState.CallerSkip() = %v, want %v", got, 2)
	}
}

// TestBaseHandler_Enabled verifies the Enabled method logic.
func TestBaseHandler_Enabled(t *testing.T) {
	t.Parallel()
	opts := &handler.BaseOptions{
		Level:  handler.InfoLevel,
		Output: io.Discard,
	}
	h := newHandler(t, opts)

	tests := []struct {
		name  string
		level handler.LogLevel
		want  bool
	}{
		{"below configured", handler.DebugLevel, false},
		{"at configured", handler.InfoLevel, true},
		{"above configured", handler.WarnLevel, true},
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

// TestBaseHandler_FlagManagement verifies HasFlag and SetFlag.
func TestBaseHandler_FlagManagement(t *testing.T) {
	t.Parallel()
	opts := &handler.BaseOptions{
		Output:     io.Discard,
		WithCaller: false,
		WithTrace:  false,
	}
	h := newHandler(t, opts)

	if h.HasFlag(handler.FlagCaller) {
		t.Fatal("HasFlag(FlagCaller) = true, want false (initial)")
	}
	if h.HasFlag(handler.FlagTrace) {
		t.Fatal("HasFlag(FlagTrace) = true, want false (initial)")
	}

	// Set Caller
	h.SetFlag(handler.FlagCaller, true)
	if !h.HasFlag(handler.FlagCaller) {
		t.Error("HasFlag(FlagCaller) = false, want true (after set)")
	}
	if h.HasFlag(handler.FlagTrace) {
		t.Error("HasFlag(FlagTrace) = true, want false (caller set)")
	}

	// Set Trace
	h.SetFlag(handler.FlagTrace, true)
	if !h.HasFlag(handler.FlagCaller) {
		t.Error("HasFlag(FlagCaller) = false, want true (trace set)")
	}
	if !h.HasFlag(handler.FlagTrace) {
		t.Error("HasFlag(FlagTrace) = false, want true (after set)")
	}

	// Clear Caller
	h.SetFlag(handler.FlagCaller, false)
	if h.HasFlag(handler.FlagCaller) {
		t.Error("HasFlag(FlagCaller) = true, want false (after clear)")
	}
	if !h.HasFlag(handler.FlagTrace) {
		t.Error("HasFlag(FlagTrace) = false, want true (caller cleared)")
	}

	// Clear Trace
	h.SetFlag(handler.FlagTrace, false)
	if h.HasFlag(handler.FlagCaller) {
		t.Error("HasFlag(FlagCaller) = true, want false (trace cleared)")
	}
	if h.HasFlag(handler.FlagTrace) {
		t.Error("HasFlag(FlagTrace) = true, want false (after clear)")
	}
}

// TestBaseHandler_FlagManagement_Concurrent verifies SetFlag is atomic.
func TestBaseHandler_FlagManagement_Concurrent(t *testing.T) {
	t.Parallel()
	opts := &handler.BaseOptions{
		Output: io.Discard,
	}
	h := newHandler(t, opts)

	var wg sync.WaitGroup
	const goroutines = 100

	// Concurrently toggle FlagCaller
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(enable bool) {
			defer wg.Done()
			h.SetFlag(handler.FlagCaller, enable)
		}(i%2 == 0) // Alternate true/false
	}
	wg.Wait()

	// Concurrently toggle FlagTrace
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(enable bool) {
			defer wg.Done()
			h.SetFlag(handler.FlagTrace, enable)
		}(i%2 == 0) // Alternate true/false
	}
	wg.Wait()

	// We don't know the final state, but it should be valid.
	// We check this by reading the flags. If this panics, the test fails.
	_ = h.HasFlag(handler.FlagCaller)
	_ = h.HasFlag(handler.FlagTrace)
	t.Logf("Test finished. Final state: Caller=%v, Trace=%v", h.HasFlag(handler.FlagCaller), h.HasFlag(handler.FlagTrace))
}

// --- Test Mutable Setters ---

func TestBaseHandler_SetLevel(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		// Clone handler for isolated test
		h := newHandler(t, &handler.BaseOptions{Level: handler.InfoLevel, Output: io.Discard})
		if err := h.SetLevel(handler.WarnLevel); err != nil {
			t.Fatalf("SetLevel(WarnLevel) error = %v, want nil", err)
		}
		if got := h.Level(); got != handler.WarnLevel {
			t.Errorf("Level() = %v, want %v", got, handler.WarnLevel)
		}
		if h.Enabled(handler.InfoLevel) {
			t.Error("Enabled(InfoLevel) = true, want false")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		// Clone handler for isolated test
		h := newHandler(t, &handler.BaseOptions{Level: handler.InfoLevel, Output: io.Discard})
		err := h.SetLevel(handler.MaxLevel + 1)
		if err == nil {
			t.Fatal("SetLevel(invalid) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrInvalidLogLevel) {
			t.Fatalf("SetLevel(invalid) error = %v, want %v", err, handler.ErrInvalidLogLevel)
		}
		if got := h.Level(); got != handler.InfoLevel {
			t.Errorf("Level() = %v, want %v (should not change on error)", got, handler.InfoLevel)
		}
	})
}

func TestBaseHandler_SetOutput(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var buf1, buf2 bytes.Buffer
		h := newHandler(t, &handler.BaseOptions{Output: &buf1})

		if err := h.SetOutput(&buf2); err != nil {
			t.Fatalf("SetOutput() error = %v, want nil", err)
		}
		// Write to the handler's writer to verify swap
		_, _ = h.AtomicWriter().Write([]byte("test"))
		if buf1.Len() > 0 {
			t.Error("original writer was written to, want empty")
		}
		if buf2.String() != "test" {
			t.Errorf("new writer got %q, want %q", buf2.String(), "test")
		}
	})

	t.Run("nil writer", func(t *testing.T) {
		t.Parallel()
		var buf1 bytes.Buffer
		h := newHandler(t, &handler.BaseOptions{Output: &buf1})
		err := h.SetOutput(nil)
		if err == nil {
			t.Fatal("SetOutput(nil) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrNilWriter) {
			t.Fatalf("SetOutput(nil) error = %v, want %v", err, handler.ErrNilWriter)
		}
	})
}

func TestBaseHandler_SetCallerSkip(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, &handler.BaseOptions{Output: io.Discard, CallerSkip: 1})
		if err := h.SetCallerSkip(3); err != nil {
			t.Fatalf("SetCallerSkip(3) error = %v, want nil", err)
		}
		if got := h.CallerSkip(); got != 3 {
			t.Errorf("CallerSkip() = %v, want %v", got, 3)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, &handler.BaseOptions{Output: io.Discard, CallerSkip: 1})
		err := h.SetCallerSkip(-1)
		if err == nil {
			t.Fatal("SetCallerSkip(-1) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrInvalidSourceSkip) {
			t.Fatalf("SetCallerSkip(-1) error = %v, want %v", err, handler.ErrInvalidSourceSkip)
		}
		if got := h.CallerSkip(); got != 1 {
			t.Errorf("CallerSkip() = %v, want %v (should not change on error)", got, 1)
		}
	})
}

func TestBaseHandler_SetSeparator(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, &handler.BaseOptions{Output: io.Discard, Separator: "_"})
		if err := h.SetSeparator("::"); err != nil {
			t.Fatalf("SetSeparator(::) error = %v, want nil", err)
		}
		if got := h.Separator(); got != "::" {
			t.Errorf("Separator() = %q, want %q", got, "::")
		}
	})

	t.Run("invalid too long", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, &handler.BaseOptions{Output: io.Discard, Separator: "_"})
		longSep := "12345678901234567"
		err := h.SetSeparator(longSep)
		if err == nil {
			t.Fatalf("SetSeparator(%q) error = nil, want non-nil", longSep)
		}
		if got := h.Separator(); got != "_" {
			t.Errorf("Separator() = %q, want %q (should not change on error)", got, "_")
		}
	})
}

// TestBaseHandler_MutableSetters_Concurrent verifies setters are thread-safe.
func TestBaseHandler_MutableSetters_Concurrent(t *testing.T) {
	t.Parallel()
	h := newHandler(t, &handler.BaseOptions{Level: handler.InfoLevel, Output: io.Discard})
	var buf1, buf2 bytes.Buffer

	var wg sync.WaitGroup
	const goroutines = 100
	wg.Add(goroutines * 4)

	levels := []handler.LogLevel{handler.TraceLevel, handler.DebugLevel, handler.InfoLevel, handler.WarnLevel, handler.ErrorLevel}

	for i := 0; i < goroutines; i++ {
		i := i
		// SetLevel
		go func() {
			defer wg.Done()
			_ = h.SetLevel(levels[i%len(levels)])
		}()
		// SetOutput
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_ = h.SetOutput(&buf1)
			} else {
				_ = h.SetOutput(&buf2)
			}
		}()
		// SetCallerSkip
		go func() {
			defer wg.Done()
			_ = h.SetCallerSkip(i)
		}()
		// SetSeparator
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_ = h.SetSeparator(".")
			} else {
				_ = h.SetSeparator("_")
			}
		}()
	}
	wg.Wait()

	// Final state is unknown, but must be valid and readable
	if got := h.Level(); !handler.IsValidLogLevel(got) {
		t.Fatalf("Level() = %v, want valid level", got)
	}
	if got := h.CallerSkip(); got < 0 || got >= goroutines {
		t.Fatalf("CallerSkip() = %v, want value in [0, %d)", got, goroutines)
	}
	if got := h.Separator(); got != "." && got != "_" {
		t.Fatalf("Separator() = %q, want '.' or '_'", got)
	}
	// Write to ensure writer is valid
	if _, err := h.AtomicWriter().Write([]byte("test")); err != nil {
		t.Fatalf("AtomicWriter.Write() failed: %v", err)
	}
}

// --- Test Immutable Builders ---

func TestBaseHandler_WithLevel(t *testing.T) {
	t.Parallel()
	h1 := newHandler(t, &handler.BaseOptions{Level: handler.InfoLevel, Output: io.Discard})

	t.Run("change", func(t *testing.T) {
		t.Parallel()
		// Change
		h2, err := h1.WithLevel(handler.WarnLevel)
		if err != nil {
			t.Fatalf("WithLevel(WarnLevel) error = %v, want nil", err)
		}
		if h2 == h1 {
			t.Fatal("WithLevel(WarnLevel) returned original instance, want new")
		}
		if got := h2.Level(); got != handler.WarnLevel {
			t.Errorf("h2.Level() = %v, want %v", got, handler.WarnLevel)
		}
		if got := h1.Level(); got != handler.InfoLevel {
			t.Errorf("h1.Level() = %v, want %v (original mutated)", got, handler.InfoLevel)
		}
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()
		h3, err := h1.WithLevel(handler.InfoLevel)
		if err != nil {
			t.Fatalf("WithLevel(InfoLevel) error = %v, want nil", err)
		}
		if h3 != h1 {
			t.Error("WithLevel(InfoLevel) returned new instance, want original")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		h4, err := h1.WithLevel(handler.MaxLevel + 1)
		if err == nil {
			t.Fatal("WithLevel(invalid) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrInvalidLogLevel) {
			t.Fatalf("WithLevel(invalid) error = %v, want %v", err, handler.ErrInvalidLogLevel)
		}
		if h4 != nil {
			t.Errorf("WithLevel(invalid) returned %v, want nil", h4)
		}
	})
}

func TestBaseHandler_WithCaller(t *testing.T) {
	t.Parallel()
	h1 := newHandler(t, &handler.BaseOptions{Output: io.Discard, WithCaller: false})

	t.Run("enable", func(t *testing.T) {
		t.Parallel()
		h2 := h1.WithCaller(true)
		if h2 == h1 {
			t.Fatal("WithCaller(true) returned original instance, want new")
		}
		if !h2.CallerEnabled() {
			t.Error("h2.CallerEnabled() = false, want true")
		}
		if h1.CallerEnabled() {
			t.Error("h1.CallerEnabled() = true, want false (original mutated)")
		}
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()
		h3 := h1.WithCaller(false)
		if h3 != h1 {
			t.Error("WithCaller(false) returned new instance, want original")
		}
	})
}

func TestBaseHandler_WithTrace(t *testing.T) {
	t.Parallel()
	h1 := newHandler(t, &handler.BaseOptions{Output: io.Discard, WithTrace: false})

	t.Run("enable", func(t *testing.T) {
		t.Parallel()
		h2 := h1.WithTrace(true)
		if h2 == h1 {
			t.Fatal("WithTrace(true) returned original instance, want new")
		}
		if !h2.TraceEnabled() {
			t.Error("h2.TraceEnabled() = false, want true")
		}
		if h1.TraceEnabled() {
			t.Error("h1.TraceEnabled() = true, want false (original mutated)")
		}
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()
		h3 := h1.WithTrace(false)
		if h3 != h1 {
			t.Error("WithTrace(false) returned new instance, want original")
		}
	})
}

func TestBaseHandler_WithKeyPrefix(t *testing.T) {
	t.Parallel()
	h1 := newHandler(t, &handler.BaseOptions{Output: io.Discard, Separator: "_"})
	t.Run("initial prefix", func(t *testing.T) {
		t.Parallel()
		h2 := h1.WithKeyPrefix("group1")
		if h2 == h1 {
			t.Fatal("WithKeyPrefix(group1) returned original instance, want new")
		}
		if got := h2.KeyPrefix(); got != "group1" {
			t.Errorf("h2.KeyPrefix() = %q, want %q", got, "group1")
		}
		if got := h1.KeyPrefix(); got != "" {
			t.Errorf("h1.KeyPrefix() = %q, want %q (original mutated)", got, "")
		}
	})

	t.Run("append prefix", func(t *testing.T) {
		t.Parallel()
		h2 := h1.WithKeyPrefix("group1")
		h3 := h2.WithKeyPrefix("group2")
		if h3 == h2 {
			t.Fatal("WithKeyPrefix(group2) returned original instance, want new")
		}
		if got := h3.KeyPrefix(); got != "group1_group2" {
			t.Errorf("h3.KeyPrefix() = %q, want %q", got, "group1_group2")
		}
		if got := h2.KeyPrefix(); got != "group1" {
			t.Errorf("h2.KeyPrefix() = %q, want %q (original mutated)", got, "group1")
		}
	})
}

func TestBaseHandler_WithCallerSkip(t *testing.T) {
	t.Parallel()
	h1 := newHandler(t, &handler.BaseOptions{Output: io.Discard, CallerSkip: 1})

	t.Run("change", func(t *testing.T) {
		t.Parallel()
		h2, err := h1.WithCallerSkip(3)
		if err != nil {
			t.Fatalf("WithCallerSkip(3) error = %v, want nil", err)
		}
		if h2 == h1 {
			t.Fatal("WithCallerSkip(3) returned original instance, want new")
		}
		if got := h2.CallerSkip(); got != 3 {
			t.Errorf("h2.CallerSkip() = %v, want %v", got, 3)
		}
		if got := h1.CallerSkip(); got != 1 {
			t.Errorf("h1.CallerSkip() = %v, want %v (original mutated)", got, 1)
		}
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()
		h3, err := h1.WithCallerSkip(1)
		if err != nil {
			t.Fatalf("WithCallerSkip(1) error = %v, want nil", err)
		}
		if h3 != h1 {
			t.Error("WithCallerSkip(1) returned new instance, want original")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		h4, err := h1.WithCallerSkip(-1)
		if err == nil {
			t.Fatal("WithCallerSkip(-1) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrInvalidSourceSkip) {
			t.Fatalf("WithCallerSkip(-1) error = %v, want %v", err, handler.ErrInvalidSourceSkip)
		}
		if h4 != nil {
			t.Errorf("WithCallerSkip(-1) returned %v, want nil", h4)
		}
	})
}

func TestBaseHandler_WithCallerSkipDelta(t *testing.T) {
	t.Parallel()
	h1 := newHandler(t, &handler.BaseOptions{Output: io.Discard, CallerSkip: 1})

	t.Run("change positive", func(t *testing.T) {
		t.Parallel()
		h2, err := h1.WithCallerSkipDelta(2) // 1 + 2 = 3
		if err != nil {
			t.Fatalf("WithCallerSkipDelta(2) error = %v, want nil", err)
		}
		if h2 == h1 {
			t.Fatal("WithCallerSkipDelta(2) returned original instance, want new")
		}
		if got := h2.CallerSkip(); got != 3 {
			t.Errorf("h2.CallerSkip() = %v, want %v", got, 3)
		}
		if got := h1.CallerSkip(); got != 1 {
			t.Errorf("h1.CallerSkip() = %v, want %v (original mutated)", got, 1)
		}
	})

	t.Run("change negative", func(t *testing.T) {
		t.Parallel()
		h3, err := h1.WithCallerSkipDelta(-1) // 1 - 1 = 0
		if err != nil {
			t.Fatalf("WithCallerSkipDelta(-1) error = %v, want nil", err)
		}
		if h3 == h1 {
			t.Fatal("WithCallerSkipDelta(-1) returned original instance, want new")
		}
		if got := h3.CallerSkip(); got != 0 {
			t.Errorf("h3.CallerSkip() = %v, want %v", got, 0)
		}
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()
		h4, err := h1.WithCallerSkipDelta(0)
		if err != nil {
			t.Fatalf("WithCallerSkipDelta(0) error = %v, want nil", err)
		}
		if h4 != h1 {
			t.Error("WithCallerSkipDelta(0) returned new instance, want original")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		h5, err := h1.WithCallerSkipDelta(-2) // 1 - 2 = -1
		if err == nil {
			t.Fatal("WithCallerSkipDelta(-2) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrInvalidSourceSkip) {
			t.Fatalf("WithCallerSkipDelta(-2) error = %v, want %v", err, handler.ErrInvalidSourceSkip)
		}
		if h5 != nil {
			t.Errorf("WithCallerSkipDelta(-2) returned %v, want nil", h5)
		}
	})
}

func TestBaseHandler_WithOutput(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var buf1, buf2 bytes.Buffer
		h1 := newHandler(t, &handler.BaseOptions{Output: &buf1})

		h2, err := h1.WithOutput(&buf2)
		if err != nil {
			t.Fatalf("WithOutput() error = %v, want nil", err)
		}
		if h2 == h1 {
			t.Fatal("WithOutput() returned original instance, want new")
		}

		// Write to verify (This is the fixed test)
		_, _ = h2.AtomicWriter().Write([]byte("h2"))
		if buf2.String() != "h2" {
			t.Errorf("h2 writer got %q, want %q", buf2.String(), "h2")
		}
		_, _ = h1.AtomicWriter().Write([]byte("h1"))
		if buf1.String() != "h1" {
			t.Errorf("h1 writer got %q, want %q", buf1.String(), "h1")
		}
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		h1 := newHandler(t, &handler.BaseOptions{Output: io.Discard})
		h3, err := h1.WithOutput(nil)
		if err == nil {
			t.Fatal("WithOutput(nil) error = nil, want non-nil")
		}
		if !errors.Is(err, handler.ErrNilWriter) {
			t.Fatalf("WithOutput(nil) error = %v, want %v", err, handler.ErrNilWriter)
		}
		if h3 != nil {
			t.Errorf("WithOutput(nil) returned %v, want nil", h3)
		}
	})
}

// --- Test Utilities ---

// TestBaseHandler_ApplyPrefix verifies key prefixing logic.
func TestBaseHandler_ApplyPrefix(t *testing.T) {
	t.Parallel()
	opts := &handler.BaseOptions{
		Output:    io.Discard,
		Separator: "::",
	}
	h := newHandler(t, opts)

	t.Run("no prefix", func(t *testing.T) {
		t.Parallel()
		if got := h.ApplyPrefix("key"); got != "key" {
			t.Errorf("ApplyPrefix(key) = %q, want %q", got, "key")
		}
	})

	t.Run("one prefix", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, opts)
		h = h.WithKeyPrefix("group1")
		if got := h.ApplyPrefix("key"); got != "group1::key" {
			t.Errorf("ApplyPrefix(key) = %q, want %q", got, "group1::key")
		}
	})

	t.Run("nested prefix", func(t *testing.T) {
		t.Parallel()
		h := newHandler(t, opts)
		h = h.WithKeyPrefix("group1")
		h = h.WithKeyPrefix("group2")
		if got := h.ApplyPrefix("key"); got != "group1::group2::key" {
			t.Errorf("ApplyPrefix(key) = %q, want %q", got, "group1::group2::key")
		}
	})
}

// TestBaseHandler_ReadState verifies the read-lock utility.
func TestBaseHandler_ReadState(t *testing.T) {
	t.Parallel()
	h := newHandler(t, &handler.BaseOptions{Output: io.Discard, Separator: "_"})

	var wg sync.WaitGroup
	var readSeparator string
	var readPrefix string

	// Start a ReadState func that blocks until we release it
	wg.Add(1)
	go func() {
		h.ReadState(func() {
			// This read should be atomic with the one below
			readSeparator = h.Separator()
			readPrefix = h.KeyPrefix()
			wg.Done() // Signal that ReadState has finished
		})
	}()

	wg.Wait() // Wait for ReadState to acquire the lock and finish

	// Try to write (this should no longer block)
	writeDone := make(chan struct{})
	go func() {
		_ = h.SetSeparator("::")
		h = h.WithKeyPrefix("test") // This will need a RLock, then Lock
		close(writeDone)
	}()

	<-writeDone // Wait for write to complete

	// At this point, ReadState has run. We can check its values.
	if readSeparator != "_" {
		t.Errorf("readSeparator = %q, want %q", readSeparator, "_")
	}
	if readPrefix != "" {
		t.Errorf("readPrefix = %q, want %q", readPrefix, "")
	}

	// Verify the write succeeded after ReadState released the lock
	if h.Separator() != "::" {
		t.Errorf("Separator() = %q, want %q", h.Separator(), "::")
	}
}
