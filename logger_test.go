package unilog_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/balinomad/go-unilog"
	"github.com/balinomad/go-unilog/handler"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()
	h := newMockHandler()

	t.Run("NewLogger success", func(t *testing.T) {
		l, err := unilog.NewLogger(h)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l == nil {
			t.Fatal("expected logger, got nil")
		}
	})

	t.Run("NewAdvancedLogger success", func(t *testing.T) {
		l, err := unilog.NewAdvancedLogger(h)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l == nil {
			t.Fatal("expected advanced logger, got nil")
		}
	})

	t.Run("NewLogger nil handler", func(t *testing.T) {
		_, err := unilog.NewLogger(nil)
		if err == nil {
			t.Error("expected error with nil handler")
		}
	})
}

func TestNewLogger_Invariants(t *testing.T) {
	t.Parallel()
	h := newMockHandler()

	t.Run("Panic on negative skip", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on negative skip")
			}
		}()
		// Direct internal call to bypass public API clamping
		unilog.XNewLogger(h, -1)
	})

	t.Run("Panic on nil HandlerState", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on nil HandlerState")
			}
		}()
		badH := &handlerWithNilState{h}
		unilog.XNewLogger(badH, 0)
	})
}

type handlerWithNilState struct{ *mockFullHandler }

func (h *handlerWithNilState) HandlerState() handler.HandlerState { return nil }

func TestLogger_Log(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		method    func(unilog.Logger, context.Context, string, ...any)
		level     unilog.LogLevel
		keyValues []any
	}{
		{
			name:      "Trace",
			method:    func(l unilog.Logger, c context.Context, m string, kv ...any) { l.Trace(c, m, kv...) },
			level:     unilog.TraceLevel,
			keyValues: nil,
		},
		{
			name:      "Debug",
			method:    func(l unilog.Logger, c context.Context, m string, kv ...any) { l.Debug(c, m, kv...) },
			level:     unilog.DebugLevel,
			keyValues: nil,
		},
		{
			name:      "Info",
			method:    func(l unilog.Logger, c context.Context, m string, kv ...any) { l.Info(c, m, kv...) },
			level:     unilog.InfoLevel,
			keyValues: nil,
		},
		{
			name:      "Warn",
			method:    func(l unilog.Logger, c context.Context, m string, kv ...any) { l.Warn(c, m, kv...) },
			level:     unilog.WarnLevel,
			keyValues: nil,
		},
		{
			name:      "Error",
			method:    func(l unilog.Logger, c context.Context, m string, kv ...any) { l.Error(c, m, kv...) },
			level:     unilog.ErrorLevel,
			keyValues: nil,
		},
		{
			name:      "Critical",
			method:    func(l unilog.Logger, c context.Context, m string, kv ...any) { l.Critical(c, m, kv...) },
			level:     unilog.CriticalLevel,
			keyValues: nil,
		},
		{
			name:      "Log",
			method:    func(l unilog.Logger, c context.Context, m string, kv ...any) { l.Log(c, unilog.InfoLevel, m, kv...) },
			level:     unilog.InfoLevel,
			keyValues: []any{"k", "v"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMockHandler()
			l, _ := unilog.NewLogger(h)

			// Retrieve the wrapped handler (NewLogger clones it)
			wh := getMockHandler(t, l)

			ctx := context.Background()
			msg := "test message"

			tt.method(l, ctx, msg, tt.keyValues...)

			r := wh.LastRecord()
			if r == nil {
				t.Fatal("handler not called")
			}
			if r.Level != tt.level {
				t.Errorf("got level %v, want %v", r.Level, tt.level)
			}
			if r.Message != msg {
				t.Errorf("got message %q, want %q", r.Message, msg)
			}
			if len(tt.keyValues) > 0 && len(r.KeyValues) == 0 {
				t.Error("attributes missing")
			}
		})
	}
}

func TestLogger_CallerCapture(t *testing.T) {
	// Not parallel: sensitive to stack depth
	tests := []struct {
		name           string
		nativeCaller   bool // Handler supports native caller (e.g. slog)
		callerEnabled  bool // Handler state says caller enabled
		expectPC       bool // Logger should capture PC
		expectSkip     bool // Logger should pass Skip
		logWithSkipVal int  // explicit skip delta
	}{
		{
			name:          "legacy handler, no caller",
			nativeCaller:  false,
			callerEnabled: false,
			expectPC:      false,
			expectSkip:    false,
		},
		{
			name:          "legacy handler, caller enabled",
			nativeCaller:  false,
			callerEnabled: true,
			expectPC:      true, // Logger captures PC manually
			expectSkip:    false,
		},
		{
			name:          "native handler, caller enabled",
			nativeCaller:  true,
			callerEnabled: true,
			expectPC:      false,
			expectSkip:    true, // Logger passes skip to handler
		},
		{
			name:           "native handler, with extra skip",
			nativeCaller:   true,
			callerEnabled:  true,
			logWithSkipVal: 2,
			expectPC:       false,
			expectSkip:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMockHandler()
			feats := handler.Feature(0)
			if tt.nativeCaller {
				feats = handler.FeatNativeCaller
			}
			h.features = handler.NewHandlerFeatures(feats)
			h.state = &mockHandlerState{caller: tt.callerEnabled}

			l, _ := unilog.NewAdvancedLogger(h)
			wh := getMockHandler(t, l)

			if tt.logWithSkipVal > 0 {
				l.LogWithSkip(context.Background(), unilog.InfoLevel, "msg", tt.logWithSkipVal)
			} else {
				l.Info(context.Background(), "msg")
			}

			r := wh.LastRecord()
			if r == nil {
				t.Fatal("handler not called")
			}

			if tt.expectPC && r.PC == 0 {
				t.Error("expected PC to be captured, got 0")
			}
			if !tt.expectPC && r.PC != 0 {
				t.Error("expected PC to be 0, got capture")
			}

			if tt.expectSkip {
				expectedSkip := unilog.XInternalSkipFrames + tt.logWithSkipVal
				if r.Skip != expectedSkip {
					t.Errorf("expected Skip %d, got %d", expectedSkip, r.Skip)
				}
			}
		})
	}
}

func TestLogger_Delegation(t *testing.T) {
	t.Parallel()
	h := newMockHandler()
	l, _ := unilog.NewAdvancedLogger(h)

	// Helper to check if operation exists in history
	hasOp := func(mh *mockFullHandler, op string) bool {
		for _, o := range mh.History() {
			if o == op {
				return true
			}
		}
		return false
	}

	t.Run("With", func(t *testing.T) {
		l2 := l.With("k", "v")
		if l2 == l {
			t.Error("expected new logger")
		}
		wh := getMockHandler(t, l2)
		// "WithCallerSkip" might appear after "WithAttrs" due to NewLogger internals
		if !hasOp(wh, "WithAttrs") {
			t.Errorf("expected WithAttrs in history %v", wh.History())
		}
	})

	t.Run("WithGroup", func(t *testing.T) {
		l2 := l.WithGroup("g")
		wh := getMockHandler(t, l2)
		if !hasOp(wh, "WithGroup") {
			t.Errorf("expected WithGroup in history %v", wh.History())
		}
	})

	t.Run("WithCallerSkip", func(t *testing.T) {
		// Base skip is unilog.XInternalSkipFrames
		targetSkip := 5
		l2 := l.WithCallerSkip(targetSkip)
		wh := getMockHandler(t, l2)

		// WithCallerSkip(5) -> mock sees 5 + internalSkipFrames (if using adj)
		expected := targetSkip + unilog.XInternalSkipFrames
		if wh.LastOp() != "WithCallerSkip" || wh.LastVal() != expected {
			t.Errorf("expected WithCallerSkip(%d), got %v(%v)", expected, wh.LastOp(), wh.LastVal())
		}
	})

	t.Run("WithCallerSkipDelta", func(t *testing.T) {
		// Base is internalSkipFrames. Delta 2 -> internalSkipFrames + 2.
		l2 := l.WithCallerSkipDelta(2)
		wh := getMockHandler(t, l2)
		// WithCallerSkipDelta(2) -> calls WithCallerSkip(internalSkipFrames + 2) on internal adj
		// BUT logger.WithCallerSkipDelta implements it by calculating absolute
		// and calling WithCallerSkip.
		// currentSkip = internalSkipFrames.
		// l.WithCallerSkip(internal - internal + 2) = l.WithCallerSkip(2)
		// NewLogger(adj.WithCallerSkip(2 + internal))
		expected := 2 + unilog.XInternalSkipFrames
		if wh.LastOp() != "WithCallerSkip" || wh.LastVal() != expected {
			t.Errorf("expected WithCallerSkip(%d), got %v(%v)", expected, wh.LastOp(), wh.LastVal())
		}
	})

	t.Run("WithCaller", func(t *testing.T) {
		l2 := l.WithCaller(true)
		wh := getMockHandler(t, l2)
		if !hasOp(wh, "WithCaller") {
			t.Errorf("expected WithCaller in history %v", wh.History())
		}
	})

	t.Run("WithTrace", func(t *testing.T) {
		l2 := l.WithTrace(true)
		wh := getMockHandler(t, l2)
		if !hasOp(wh, "WithTrace") {
			t.Errorf("expected WithTrace in history %v", wh.History())
		}
	})

	t.Run("WithLevel", func(t *testing.T) {
		l2 := l.WithLevel(unilog.WarnLevel)
		wh := getMockHandler(t, l2)
		if !hasOp(wh, "WithLevel") {
			t.Errorf("expected WithLevel in history %v", wh.History())
		}
	})

	t.Run("WithOutput", func(t *testing.T) {
		l2 := l.WithOutput(io.Discard)
		wh := getMockHandler(t, l2)
		if !hasOp(wh, "WithOutput") {
			t.Errorf("expected WithOutput in history %v", wh.History())
		}
	})
}

func TestLogger_MissingCapabilities(t *testing.T) {
	t.Parallel()
	// Use the wrapper that only implements basic Handler interface
	mh := newMockMinimalHandler()
	l, _ := unilog.NewLogger(mh)
	adv, _ := unilog.NewAdvancedLogger(mh)
	mut, _ := l.(unilog.MutableLogger)

	t.Run("Mutable No-Ops", func(t *testing.T) {
		if err := mut.SetLevel(unilog.ErrorLevel); err != nil {
			t.Error("expected no error on missing capability")
		}
		if err := mut.SetOutput(io.Discard); err != nil {
			t.Error("expected no error on missing capability")
		}
	})

	t.Run("Advanced No-Ops or Fallbacks", func(t *testing.T) {
		// Sync should be no-op
		if err := adv.Sync(); err != nil {
			t.Error("expected no error on missing Sync")
		}

		// With* methods should return original instance (optimization)
		// or safe clones if not supported but state needs preservation
		if adv.WithLevel(unilog.InfoLevel) != adv {
			t.Error("expected same instance when Configurable not supported")
		}
		if adv.WithOutput(io.Discard) != adv {
			t.Error("expected same instance when Configurable not supported")
		}
		if adv.WithCaller(true) != adv {
			t.Error("expected same instance when FeatureToggler not supported")
		}
		if adv.WithTrace(true) != adv {
			t.Error("expected same instance when FeatureToggler not supported")
		}
		if adv.With("k", "v") != adv {
			t.Error("expected same instance when Chainer not supported")
		}
		if adv.WithGroup("g") != adv {
			t.Error("expected same instance when Chainer not supported")
		}
	})

	t.Run("WithCallerSkip Fallback", func(t *testing.T) {
		// WithCallerSkip should still work by returning a NEW logger
		// with the skip value updated, even if handler doesn't support adjustment.
		// It just wraps the same handler with a new skip offset.
		l2 := adv.WithCallerSkip(5)
		if l2 == adv {
			t.Error("expected new logger for WithCallerSkip")
		}
		// Cannot verify handler internal state easily here as it is opaque,
		// but we verified it returns a new instance.
	})
}

func TestLogger_Mutable(t *testing.T) {
	t.Parallel()
	h := newMockHandler()
	l, _ := unilog.NewLogger(h)
	// Mutable methods operate on the handler instance directly
	// But since NewLogger cloned it, we must check the cloned instance.
	wh := getMockHandler(t, l)

	mut, ok := l.(unilog.MutableLogger)
	if !ok {
		t.Fatal("expected MutableLogger")
	}

	t.Run("SetLevel", func(t *testing.T) {
		err := mut.SetLevel(unilog.ErrorLevel)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if wh.LastOp() != "SetLevel" || wh.LastVal() != unilog.ErrorLevel {
			t.Errorf("expected SetLevel(Error), got %v(%v)", wh.LastOp(), wh.LastVal())
		}
	})

	t.Run("SetOutput", func(t *testing.T) {
		err := mut.SetOutput(io.Discard)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if wh.LastOp() != "SetOutput" {
			t.Errorf("expected SetOutput, got %v", wh.LastOp())
		}
	})
}

func TestLogger_Sync(t *testing.T) {
	t.Parallel()
	h := newMockHandler()
	l, _ := unilog.NewAdvancedLogger(h)
	wh := getMockHandler(t, l)

	if err := l.Sync(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if wh.LastOp() != "Sync" {
		t.Error("expected Sync called")
	}

	wh.mu.Lock()
	wh.errSync = errors.New("sync fail")
	wh.mu.Unlock()

	if err := l.Sync(); err == nil {
		t.Error("expected sync error")
	}
}

func TestLogger_Fatal_Panic_Process(t *testing.T) {
	// Uses sub-process execution to check os.Exit(1)
	if os.Getenv("TEST_LOGGER_FATAL") == "1" {
		h := newMockHandler()
		l, _ := unilog.NewLogger(h)
		l.Fatal(context.Background(), "die")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestLogger_Fatal_Panic_Process")
	cmd.Env = append(os.Environ(), "TEST_LOGGER_FATAL=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		// Success, it exited with non-zero
	} else {
		t.Fatalf("process ran with err %v, want exit status 1", err)
	}
}

func TestLogger_Panic_Recovery(t *testing.T) {
	t.Parallel()
	h := newMockHandler()
	l, _ := unilog.NewLogger(h)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		} else {
			if r != "panic msg" {
				t.Errorf("expected panic 'panic msg', got %v", r)
			}
		}
	}()

	l.Panic(context.Background(), "panic msg")
}

func TestLogger_AttributeNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		keyValues []any
		wantCount int
	}{
		{
			name:      "no attributes",
			keyValues: []any{},
			wantCount: 0,
		},
		{
			name:      "single pair",
			keyValues: []any{"key", "val"},
			wantCount: 2,
		},
		{
			name:      "multiple pairs",
			keyValues: []any{"k1", "v1", "k2", 42, "k3", true},
			wantCount: 6,
		},
		{
			name:      "odd count ignored",
			keyValues: []any{"k1", "v1", "k2"},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMockHandler()
			l, _ := unilog.NewLogger(h)
			wh := getMockHandler(t, l)

			l.Info(context.Background(), "msg", tt.keyValues...)

			r := wh.LastRecord()
			if r == nil {
				t.Fatal("handler not called")
			}
			if len(r.KeyValues) != tt.wantCount {
				t.Errorf("keyvalues count = %d, want %d", len(r.KeyValues), tt.wantCount)
			}
		})
	}
}

func TestLogger_ContextCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ctxFunc   func() context.Context
		shouldLog bool
	}{
		{
			name:      "background context logs",
			ctxFunc:   context.Background,
			shouldLog: true,
		},
		{
			name: "canceled context skips",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			shouldLog: false,
		},
		{
			name:      "nil context logs",
			ctxFunc:   func() context.Context { return nil },
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMockHandler()
			l, _ := unilog.NewLogger(h)
			wh := getMockHandler(t, l)

			l.Info(tt.ctxFunc(), "msg")

			r := wh.LastRecord()
			if tt.shouldLog && r == nil {
				t.Error("expected log, got none")
			}
			if !tt.shouldLog && r != nil {
				t.Error("expected no log, got one")
			}
		})
	}
}

func TestLogger_Enabled(t *testing.T) {
	t.Parallel()

	h := newMockHandler()
	l, _ := unilog.NewLogger(h)
	wh := getMockHandler(t, l)

	// Mock handler is enabled by default
	if !l.Enabled(unilog.InfoLevel) {
		t.Error("should be enabled at InfoLevel")
	}

	wh.mu.Lock()
	wh.enabled = false
	wh.mu.Unlock()

	if l.Enabled(unilog.InfoLevel) {
		t.Error("should be disabled when handler is disabled")
	}
}

func TestLogger_With_Optimization(t *testing.T) {
	t.Parallel()

	h := newMockHandler()
	l, _ := unilog.NewLogger(h)

	if l.With() != l {
		t.Error("With() with no args should return same logger")
	}

	if l.With("k", "v") == l {
		t.Error("With(k,v) should return new logger")
	}

}

func TestLogger_WithGroup_Optimization(t *testing.T) {
	t.Parallel()

	h := newMockHandler()
	l, _ := unilog.NewLogger(h)

	if l.WithGroup("") != l {
		t.Error("WithGroup(\"\") should return same logger")
	}

	if l.WithGroup("g") == l {
		t.Error("WithGroup(\"g\") should return new logger")
	}
}

func TestLogger_Timestamp(t *testing.T) {
	t.Parallel()

	h := newMockHandler()
	l, _ := unilog.NewLogger(h)
	wh := getMockHandler(t, l)

	before := time.Now()
	l.Info(context.Background(), "msg")
	after := time.Now()

	r := wh.LastRecord()
	if r == nil {
		t.Fatal("handler not called")
	}

	// Verify timestamp is within the test execution window
	if r.Time.Before(before.Add(-time.Millisecond)) || r.Time.After(after.Add(time.Millisecond)) {
		t.Errorf("timestamp %v out of range [%v, %v]", r.Time, before, after)
	}
}

func TestLogger_Concurrent(t *testing.T) {
	t.Parallel()

	h := newMockHandler()
	l, _ := unilog.NewLogger(h)
	wh := getMockHandler(t, l)

	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 100
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			l.Info(ctx, fmt.Sprintf("msg %d", idx), "id", idx)
		}(i)
	}
	wg.Wait()

	if count := wh.CallCount(); count != goroutines {
		t.Errorf("expected %d calls, got %d", goroutines, count)
	}
}

func TestLogger_FallbackOnHandlerError(t *testing.T) {
	t.Parallel()

	h := newMockHandler()
	l, _ := unilog.NewLogger(h)
	wh := getMockHandler(t, l)

	wh.mu.Lock()
	wh.errHandle = errors.New("fail")
	wh.mu.Unlock()

	// Should not panic; calls fallback logger internally
	// We can't easily intercept the fallback logger output here without
	// mocking global fallback, but we can ensure it doesn't panic.
	l.Info(context.Background(), "test")

	if wh.CallCount() != 1 {
		t.Error("handler should have been called")
	}
}

func getMockHandler(t *testing.T, l unilog.Logger) *mockFullHandler {
	t.Helper()
	h := unilog.XLoggerHandler(l)
	if h == nil {
		t.Fatal("failed to retrieve handler from logger")
	}
	// If it's a minimal wrapper, extract target
	if mw, ok := h.(*mockMinimalWrapper); ok {
		return mw.target
	}
	mh, ok := h.(*mockFullHandler)
	if !ok {
		t.Fatalf("handler is %T, want *mockFullHandler", h)
	}
	return mh
}
