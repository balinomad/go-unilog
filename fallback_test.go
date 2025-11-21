package unilog_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/balinomad/go-unilog"
	"github.com/balinomad/go-unilog/handler"
)

func TestNewFallbackLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		writer  io.Writer
		level   unilog.LogLevel
		wantErr error
	}{
		{
			name:    "valid info",
			writer:  io.Discard,
			level:   unilog.InfoLevel,
			wantErr: nil,
		},
		{
			name:    "nil writer",
			writer:  nil,
			level:   unilog.InfoLevel,
			wantErr: unilog.ErrNilWriter,
		},
		{
			name:    "invalid level",
			writer:  io.Discard,
			level:   handler.MaxLevel + 1,
			wantErr: unilog.ErrInvalidLogLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := unilog.XNewFallbackLogger(tt.writer, tt.level)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
			if err == nil && l == nil {
				t.Error("got nil logger on success")
			}
		})
	}
}

func TestNewSimpleFallbackLogger(t *testing.T) {
	// Cannot run parallel due to os.Stderr manipulation
	l := unilog.XNewSimpleFallbackLogger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	if !l.Enabled(unilog.InfoLevel) {
		t.Error("logger should be enabled at InfoLevel by default")
	}
}

func TestNewSimpleFallbackLogger_Panic(t *testing.T) {
	// Cannot run parallel due to os.Stderr manipulation
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	os.Stderr = nil

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when os.Stderr is nil")
		}
	}()

	unilog.XNewSimpleFallbackLogger()
}

func TestFallbackLogger_LogOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		minLevel   unilog.LogLevel
		logOp      func(unilog.Logger)
		wantOutput string
		wantEmpty  bool
	}{
		{
			name:     "info logged",
			minLevel: unilog.InfoLevel,
			logOp: func(l unilog.Logger) {
				l.Info(context.Background(), "hello", "k", "v")
			},
			wantOutput: "INFO: hello k=v",
		},
		{
			name:     "debug skipped",
			minLevel: unilog.InfoLevel,
			logOp: func(l unilog.Logger) {
				l.Debug(context.Background(), "hello")
			},
			wantEmpty: true,
		},
		{
			name:     "trace method",
			minLevel: unilog.TraceLevel,
			logOp: func(l unilog.Logger) {
				l.Trace(context.Background(), "trace")
			},
			wantOutput: "TRACE: trace",
		},
		{
			name:     "warn method",
			minLevel: unilog.InfoLevel,
			logOp: func(l unilog.Logger) {
				l.Warn(context.Background(), "warn")
			},
			wantOutput: "WARN: warn",
		},
		{
			name:     "error method",
			minLevel: unilog.InfoLevel,
			logOp: func(l unilog.Logger) {
				l.Error(context.Background(), "err")
			},
			wantOutput: "ERROR: err",
		},
		{
			name:     "critical method",
			minLevel: unilog.InfoLevel,
			logOp: func(l unilog.Logger) {
				l.Critical(context.Background(), "crit")
			},
			wantOutput: "CRITICAL: crit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l, _ := unilog.XNewFallbackLogger(&buf, tt.minLevel)

			tt.logOp(l)

			got := buf.String()
			if tt.wantEmpty {
				if len(got) > 0 {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantOutput) {
				t.Errorf("got %q, want substring %q", got, tt.wantOutput)
			}
		})
	}
}

func TestFallbackLogger_Immutability(t *testing.T) {
	t.Parallel()
	l, _ := unilog.XNewFallbackLogger(io.Discard, unilog.InfoLevel)

	if l.With("k", "v") != l {
		t.Error("With should return same instance")
	}
	if l.WithGroup("g") != l {
		t.Error("WithGroup should return same instance")
	}
}

func TestFallbackLogger_Enabled(t *testing.T) {
	tests := []struct {
		name        string
		configLevel unilog.LogLevel
		testLevel   unilog.LogLevel
		wantEnabled bool
	}{
		{
			name:        "debug disabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.DebugLevel,
			wantEnabled: false,
		},
		{
			name:        "info disabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.InfoLevel,
			wantEnabled: false,
		},
		{
			name:        "warn enabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.WarnLevel,
			wantEnabled: true,
		},
		{
			name:        "error enabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.ErrorLevel,
			wantEnabled: true,
		},
		{
			name:        "trace enabled when trace configured",
			configLevel: unilog.TraceLevel,
			testLevel:   unilog.TraceLevel,
			wantEnabled: true,
		},
		{
			name:        "all enabled when trace configured",
			configLevel: unilog.TraceLevel,
			testLevel:   unilog.ErrorLevel,
			wantEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := unilog.XNewFallbackLogger(io.Discard, tt.configLevel)
			if err != nil {
				t.Fatalf("NewFallbackLogger() error = %v", err)
			}

			got := logger.Enabled(tt.testLevel)
			if got != tt.wantEnabled {
				t.Errorf("Enabled(%v) = %v, want %v", tt.testLevel, got, tt.wantEnabled)
			}
		})
	}
}

func TestFallbackLogger_With(t *testing.T) {
	logger, err := unilog.XNewFallbackLogger(io.Discard, unilog.InfoLevel)
	if err != nil {
		t.Fatalf("NewFallbackLogger() error = %v", err)
	}

	loggerWith := logger.With("key", "val")

	if logger != loggerWith {
		t.Error("With() should return the same instance for fallbackLogger")
	}
}

func TestFallbackLogger_WithGroup(t *testing.T) {
	logger, err := unilog.XNewFallbackLogger(io.Discard, unilog.InfoLevel)
	if err != nil {
		t.Fatalf("NewFallbackLogger() error = %v", err)
	}

	loggerWithGroup := logger.WithGroup("group")

	if logger != loggerWithGroup {
		t.Error("WithGroup() should return the same instance for fallbackLogger")
	}
}

func TestFallbackLogger_LevelMethods(t *testing.T) {
	tests := []struct {
		name      string
		method    func(unilog.Logger, context.Context, string, ...any)
		wantLevel string
	}{
		{
			name:      "trace logs at trace level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Trace(ctx, msg, kv...) },
			wantLevel: "TRACE",
		},
		{
			name:      "debug logs at debug level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Debug(ctx, msg, kv...) },
			wantLevel: "DEBUG",
		},
		{
			name:      "info logs at info level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Info(ctx, msg, kv...) },
			wantLevel: "INFO",
		},
		{
			name:      "warn logs at warn level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Warn(ctx, msg, kv...) },
			wantLevel: "WARN",
		},
		{
			name:      "error logs at error level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Error(ctx, msg, kv...) },
			wantLevel: "ERROR",
		},
		{
			name:      "critical logs at critical level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Critical(ctx, msg, kv...) },
			wantLevel: "CRITICAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger, err := unilog.XNewFallbackLogger(buf, handler.MinLevel)
			if err != nil {
				t.Fatalf("NewFallbackLogger() error = %v", err)
			}

			ctx := context.Background()
			tt.method(logger, ctx, "message")

			output := buf.String()
			expectedOutput := tt.wantLevel + ": message"
			if !strings.Contains(output, expectedOutput) {
				t.Errorf("expected output to contain %q, got: %s", expectedOutput, output)
			}
		})
	}
}

// Test for Fatal's call to os.Exit. This is a standard way to test this.
// It re-runs the test with a specific environment variable set.
func TestFallbackLogger_Fatal(t *testing.T) {
	if os.Getenv("BE_FATAL") == "1" {
		l, _ := unilog.XNewFallbackLogger(io.Discard, unilog.InfoLevel)
		l.Fatal(context.Background(), "bye")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFallbackLogger_Fatal")
	cmd.Env = append(os.Environ(), "BE_FATAL=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return // Success
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

// Test for Panic's call to panic.
func TestFallbackLogger_Panic(t *testing.T) {
	t.Parallel()

	l, _ := unilog.XNewFallbackLogger(io.Discard, unilog.InfoLevel)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	l.Panic(context.Background(), "panic")
}

func TestFallbackLogger_NilContext(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := unilog.XNewFallbackLogger(buf, unilog.InfoLevel)
	if err != nil {
		t.Fatalf("NewFallbackLogger() error = %v", err)
	}

	logger.Info(nil, "test with nil context")

	if buf.Len() == 0 {
		t.Error("expected log output with nil context")
	}
}

func TestFallbackLogger_Concurrency(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l, _ := unilog.XNewFallbackLogger(&buf, unilog.InfoLevel)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.Info(context.Background(), fmt.Sprintf("msg %d", n))
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 50 {
		t.Errorf("expected 50 lines, got %d", len(lines))
	}
}

func TestFallbackLogger_EmptyMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, _ := unilog.XNewFallbackLogger(buf, unilog.InfoLevel)
	logger.Info(context.Background(), "")

	if buf.Len() == 0 {
		t.Error("empty message should still log")
	}
}

func TestFallbackLogger_VeryLargeKeyValues(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, _ := unilog.XNewFallbackLogger(buf, unilog.InfoLevel)

	kv := make([]any, 1000)
	for i := 0; i < 1000; i += 2 {
		kv[i] = fmt.Sprintf("k%d", i)
		kv[i+1] = i
	}

	logger.Info(context.Background(), "msg", kv...)

	if buf.Len() == 0 {
		t.Error("large keyvalues should log")
	}
}
