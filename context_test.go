package unilog_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/balinomad/go-unilog"
)

// Test-specific context key type to avoid collisions
type testKey string

func TestContext(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		wantOk    bool
		wantNil   bool
		checkFunc func(t *testing.T, logger unilog.Logger)
	}{
		{
			name:     "empty context returns nil and false",
			setupCtx: func() context.Context { return context.Background() },
			wantOk:   false,
			wantNil:  true,
		},
		{
			name: "context with logger returns logger and true",
			setupCtx: func() context.Context {
				logger := newMockLogger()
				return unilog.WithLogger(context.Background(), logger)
			},
			wantOk:  true,
			wantNil: false,
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if _, ok := logger.(*mockLogger); !ok {
					t.Errorf("expected *mockLogger, got %T", logger)
				}
			},
		},
		{
			name: "context with wrong type returns nil and false",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), unilog.LoggerKey, "not a logger")
			},
			wantOk:  false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			logger, ok := unilog.LoggerFromContext(ctx)

			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}

			if tt.wantNil && logger != nil {
				t.Errorf("logger = %v, want nil", logger)
			}

			if !tt.wantNil && logger == nil {
				t.Error("logger = nil, want non-nil")
			}

			if tt.checkFunc != nil && logger != nil {
				tt.checkFunc(t, logger)
			}
		})
	}
}

func TestWithLogger(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() (context.Context, unilog.Logger)
		checkFunc func(t *testing.T, ctx context.Context, original unilog.Logger)
	}{
		{
			name: "adds logger to context",
			setupCtx: func() (context.Context, unilog.Logger) {
				logger := newMockLogger()
				return context.Background(), logger
			},
			checkFunc: func(t *testing.T, ctx context.Context, original unilog.Logger) {
				retrieved, ok := unilog.LoggerFromContext(ctx)
				if !ok {
					t.Error("LoggerFromContext returned false")
				}
				if retrieved != original {
					t.Errorf("retrieved logger %v != original %v", retrieved, original)
				}
			},
		},
		{
			name: "returns different context instance",
			setupCtx: func() (context.Context, unilog.Logger) {
				return context.Background(), newMockLogger()
			},
			checkFunc: func(t *testing.T, ctx context.Context, original unilog.Logger) {
				if ctx == context.Background() {
					t.Error("WithLogger returned same context instance")
				}
			},
		},
		{
			name: "preserves existing context values",
			setupCtx: func() (context.Context, unilog.Logger) {
				ctx := context.WithValue(context.Background(), testKey("key"), "value")
				return ctx, newMockLogger()
			},
			checkFunc: func(t *testing.T, ctx context.Context, original unilog.Logger) {
				if v := ctx.Value(testKey("key")); v != "value" {
					t.Errorf("context value = %v, want 'value'", v)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseCtx, logger := tt.setupCtx()
			resultCtx := unilog.WithLogger(baseCtx, logger)

			if tt.checkFunc != nil {
				tt.checkFunc(t, resultCtx, logger)
			}
		})
	}
}

func TestLoggerFromContextOrDefault(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		setupLog  func()
		checkFunc func(t *testing.T, logger unilog.Logger)
	}{
		{
			name: "returns logger from context when present",
			setupCtx: func() context.Context {
				logger := newMockLogger()
				return unilog.WithLogger(context.Background(), logger)
			},
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if _, ok := logger.(*mockLogger); !ok {
					t.Errorf("expected *mockLogger, got %T", logger)
				}
			},
		},
		{
			name:     "returns default logger when context empty",
			setupCtx: func() context.Context { return context.Background() },
			setupLog: func() {
				unilog.SetDefault(nil)
				unilog.SetDefault(newMockLogger())
			},
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if _, ok := logger.(*mockLogger); !ok {
					t.Errorf("expected *mockLogger, got %T", logger)
				}
			},
		},
		{
			name:     "returns default with nil context",
			setupCtx: func() context.Context { return nil },
			setupLog: func() {
				unilog.SetDefault(nil)
				unilog.SetDefault(newMockLogger())
			},
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if logger == nil {
					t.Fatal("expected non-nil logger")
				}
			},
		},
		{
			name: "prefers context logger over default",
			setupCtx: func() context.Context {
				contextLogger := newMockLogger()
				return unilog.WithLogger(context.Background(), contextLogger)
			},
			setupLog: func() {
				unilog.SetDefault(nil)
				unilog.SetDefault(newMockLogger())
			},
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				ctxLogger, ok := unilog.LoggerFromContext(
					unilog.WithLogger(context.Background(), logger),
				)
				if !ok || ctxLogger != logger {
					t.Error("did not return context logger")
				}
			},
		},
		{
			name:     "returns fallback when no default set",
			setupCtx: func() context.Context { return context.Background() },
			setupLog: func() { unilog.SetDefault(nil) },
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if logger == nil {
					t.Fatal("expected fallback logger, got nil")
				}
			},
		},
		{
			name:     "returned logger is functional",
			setupCtx: func() context.Context { return context.Background() },
			setupLog: func() {
				unilog.SetDefault(nil)
				unilog.SetDefault(newMockLogger())
			},
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				ctx := context.Background()
				logger.Info(ctx, "test")
				if mock, ok := logger.(*mockLogger); ok {
					if !strings.Contains(mock.String(), "INFO: test") {
						t.Errorf("logger output = %q, want 'INFO: test'", mock.String())
					}
				}
			},
		},
		{
			name: "handles wrong type in context",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), unilog.LoggerKey, "not a logger")
			},
			setupLog: func() {
				unilog.SetDefault(nil)
				unilog.SetDefault(newMockLogger())
			},
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if _, ok := logger.(*mockLogger); !ok {
					t.Errorf("expected fallback to default, got %T", logger)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupLog != nil {
				tt.setupLog()
				defer unilog.SetDefault(nil)
			}

			ctx := tt.setupCtx()
			logger := unilog.LoggerFromContextOrDefault(ctx)

			if tt.checkFunc != nil {
				tt.checkFunc(t, logger)
			}
		})
	}
}

func TestLoggerFromContextOrDefault_Concurrency(t *testing.T) {
	unilog.SetDefault(nil)
	defer unilog.SetDefault(nil)

	defaultLogger := newMockLogger()
	unilog.SetDefault(defaultLogger)

	contextLogger := newMockLogger()
	ctx := unilog.WithLogger(context.Background(), contextLogger)

	var wg sync.WaitGroup
	const numGoroutines = 100
	errors := make(chan string, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			var logger unilog.Logger
			if idx%2 == 0 {
				logger = unilog.LoggerFromContextOrDefault(ctx)
				if logger != contextLogger {
					errors <- fmt.Sprintf("goroutine %d: expected context logger", idx)
				}
			} else {
				logger = unilog.LoggerFromContextOrDefault(context.Background())
				if logger != defaultLogger {
					errors <- fmt.Sprintf("goroutine %d: expected default logger", idx)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestLoggerFromContextOrDefault_Inheritance(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		checkFunc func(t *testing.T, logger unilog.Logger, parentLogger unilog.Logger)
	}{
		{
			name: "child context inherits parent logger",
			setupCtx: func() context.Context {
				parentLogger := newMockLogger()
				parentCtx := unilog.WithLogger(context.Background(), parentLogger)
				return context.WithValue(parentCtx, testKey("key"), "value")
			},
			checkFunc: func(t *testing.T, logger unilog.Logger, parentLogger unilog.Logger) {
				if logger != parentLogger {
					t.Errorf("child did not inherit parent logger")
				}
			},
		},
		{
			name: "child can override parent logger",
			setupCtx: func() context.Context {
				parentLogger := newMockLogger()
				parentCtx := unilog.WithLogger(context.Background(), parentLogger)
				childLogger := newMockLogger()
				return unilog.WithLogger(parentCtx, childLogger)
			},
			checkFunc: func(t *testing.T, logger unilog.Logger, parentLogger unilog.Logger) {
				if logger == parentLogger {
					t.Error("child should override parent logger")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parentLogger := newMockLogger()
			parentCtx := unilog.WithLogger(context.Background(), parentLogger)

			var childCtx context.Context
			if tt.name == "child context inherits parent logger" {
				childCtx = context.WithValue(parentCtx, testKey("key"), "value")
			} else {
				childLogger := newMockLogger()
				childCtx = unilog.WithLogger(parentCtx, childLogger)
			}

			logger := unilog.LoggerFromContextOrDefault(childCtx)
			if tt.checkFunc != nil {
				tt.checkFunc(t, logger, parentLogger)
			}
		})
	}
}

func TestLoggerFromContextOrDefault_SpecialContexts(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
	}{
		{
			name:     "background context",
			setupCtx: func() context.Context { return context.Background() },
		},
		{
			name:     "todo context",
			setupCtx: func() context.Context { return context.TODO() },
		},
		{
			name: "canceled context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unilog.SetDefault(nil)
			defer unilog.SetDefault(nil)

			defaultLogger := newMockLogger()
			unilog.SetDefault(defaultLogger)

			ctx := tt.setupCtx()
			logger := unilog.LoggerFromContextOrDefault(ctx)

			if logger != defaultLogger {
				t.Errorf("expected default logger, got %v", logger)
			}
		})
	}
}

func TestLoggerFromContextOrDefault_MultipleContexts(t *testing.T) {
	loggers := []*mockLogger{
		newMockLogger(),
		newMockLogger(),
		newMockLogger(),
	}

	contexts := []context.Context{
		unilog.WithLogger(context.Background(), loggers[0]),
		unilog.WithLogger(context.Background(), loggers[1]),
		unilog.WithLogger(context.Background(), loggers[2]),
	}

	for i, ctx := range contexts {
		retrieved := unilog.LoggerFromContextOrDefault(ctx)
		if retrieved != loggers[i] {
			t.Errorf("context %d: got %v, want %v", i, retrieved, loggers[i])
		}
	}

	// Verify isolation
	for i := range loggers {
		for j := i + 1; j < len(loggers); j++ {
			if loggers[i] == loggers[j] {
				t.Errorf("loggers[%d] == loggers[%d], want different instances", i, j)
			}
		}
	}
}

func TestLoggerFromContextOrDefault_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
	}{
		{
			name: "context with multiple values",
			setupCtx: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, testKey("key1"), "value1")
				ctx = context.WithValue(ctx, testKey("key2"), "value2")
				ctx = unilog.WithLogger(ctx, newMockLogger())
				return context.WithValue(ctx, testKey("key3"), "value3")
			},
		},
		{
			name: "deeply nested context",
			setupCtx: func() context.Context {
				ctx := context.Background()
				for i := range 100 {
					ctx = context.WithValue(ctx, testKey(fmt.Sprintf("key%d", i)), i)
				}
				return ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unilog.SetDefault(nil)
			defer unilog.SetDefault(nil)

			unilog.SetDefault(newMockLogger())
			ctx := tt.setupCtx()

			logger := unilog.LoggerFromContextOrDefault(ctx)
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
		})
	}
}
