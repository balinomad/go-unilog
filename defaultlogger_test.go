package unilog_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/balinomad/go-unilog"
)

// TestDefault tests the Default() function.
func TestDefault(t *testing.T) {
	tests := []struct {
		name      string
		setup     func()
		wantType  string
		wantPanic bool
	}{
		{
			name:     "creates fallback logger on first call",
			setup:    func() {},
			wantType: "*unilog.fallbackLogger",
		},
		{
			name: "returns custom logger when set",
			setup: func() {
				unilog.SetDefault(newMockLogger())
			},
			wantType: "*unilog.mockLogger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetDefault()
			defer resetDefault()

			tt.setup()

			logger := unilog.Default()
			if logger == nil {
				t.Fatal("Default() returned nil")
			}

			// Check type
			switch tt.wantType {
			case "*unilog.fallbackLogger":
				if _, ok := logger.(*unilog.FallbackLogger); !ok {
					t.Errorf("expected %s, got %T", tt.wantType, logger)
				}
			case "*unilog.mockLogger":
				if _, ok := logger.(*mockLogger); !ok {
					t.Errorf("expected %s, got %T", tt.wantType, logger)
				}
			}
		})
	}
}

// TestDefaultConcurrency tests that Default() is thread-safe.
func TestDefaultConcurrency(t *testing.T) {
	resetDefault()
	defer resetDefault()

	// Test concurrent access to Default()
	var wg sync.WaitGroup
	loggers := make([]unilog.Logger, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			loggers[idx] = unilog.Default()
		}(i)
	}

	wg.Wait()

	// All loggers should be the same instance
	first := loggers[0]
	for i := 1; i < len(loggers); i++ {
		if loggers[i] != first {
			t.Errorf("concurrent Default() call %d returned different instance", i)
		}
	}
}

// TestSetDefault tests that SetDefault sets the global default logger and
// Default returns the same instance.
func TestSetDefault(t *testing.T) {
	tests := []struct {
		name   string
		logger unilog.Logger
	}{
		{
			name:   "set mock logger",
			logger: newMockLogger(),
		},
		{
			name:   "set nil logger then default creates fallback",
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetDefault()
			defer resetDefault()

			if tt.logger != nil {
				unilog.SetDefault(tt.logger)
				got := unilog.Default()
				if got != tt.logger {
					t.Errorf("Default() = %v, want %v", got, tt.logger)
				}
			} else {
				// Setting nil should cause Default() to create fallback
				unilog.SetDefault(nil)
				got := unilog.Default()
				if _, ok := got.(*unilog.FallbackLogger); !ok {
					t.Errorf("Default() after SetDefault(nil) should return fallbackLogger, got %T", got)
				}
			}
		})
	}
}

// TestLog tests the log functions.
func TestLog(t *testing.T) {
	resetDefault()
	defer resetDefault()

	mock := newMockLogger()
	unilog.SetDefault(mock)

	ctx := context.Background()

	tests := []struct {
		name     string
		level    unilog.LogLevel
		msg      string
		keyVals  []any
		expected string
	}{
		{
			name:     "log trace message",
			level:    unilog.TraceLevel,
			msg:      "trace info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "TRACE: trace info",
		},
		{
			name:     "log debug message",
			level:    unilog.DebugLevel,
			msg:      "debug info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "DEBUG: debug info",
		},
		{
			name:     "log info message",
			level:    unilog.InfoLevel,
			msg:      "test message",
			keyVals:  []any{"key", "value"},
			expected: "INFO: test message",
		},
		{
			name:     "log warn message",
			level:    unilog.WarnLevel,
			msg:      "warn info",
			keyVals:  nil,
			expected: "WARN: warn info",
		},
		{
			name:     "log error message",
			level:    unilog.ErrorLevel,
			msg:      "error occurred",
			keyVals:  nil,
			expected: "ERROR: error occurred",
		},
		{
			name:     "log critical message",
			level:    unilog.CriticalLevel,
			msg:      "critical info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "CRITICAL: critical info",
		},
		{
			name:     "log fatal message",
			level:    unilog.FatalLevel,
			msg:      "fatal info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "FATAL: fatal info [FATAL]",
		},
		{
			name:     "log panic message",
			level:    unilog.PanicLevel,
			msg:      "panic info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "PANIC: panic info [PANIC]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.buf.Reset()
			unilog.Log(ctx, tt.level, tt.msg, tt.keyVals...)

			if got := mock.String(); got != tt.expected {
				t.Errorf("Log() output = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLogWithSkip(t *testing.T) {
	tests := []struct {
		name         string
		logger       unilog.Logger
		skip         int
		msg          string
		wantContains string
	}{
		{
			name:         "with skipper implementation",
			logger:       newMockLoggerWithSkipper(),
			skip:         1,
			msg:          "test with skip",
			wantContains: "INFO: test with skip [skip:3]", // skip+2 from logWithDefault
		},
		{
			name:         "without skipper falls back to regular log",
			logger:       newMockLogger(),
			skip:         1,
			msg:          "test without skip",
			wantContains: "INFO: test without skip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetDefault()
			defer resetDefault()

			unilog.SetDefault(tt.logger)
			ctx := context.Background()

			unilog.LogWithSkip(ctx, unilog.InfoLevel, tt.msg, tt.skip)

			var output string
			switch l := tt.logger.(type) {
			case *mockLoggerWithSkipper:
				output = l.String()
			case *mockLogger:
				output = l.String()
			}

			if !strings.Contains(output, tt.wantContains) {
				t.Errorf("LogWithSkip() output = %q, want to contain %q", output, tt.wantContains)
			}
		})
	}
}

func TestCallerSkipperPath(t *testing.T) {
	resetDefault()
	defer resetDefault()

	// Test with CallerSkipper implementation
	skipperLogger := newMockLoggerWithSkipper()
	unilog.SetDefault(skipperLogger)

	ctx := context.Background()

	// Test various skip values
	testCases := []struct {
		name     string
		logFunc  func()
		wantSkip int
	}{
		{
			name:     "direct log call",
			logFunc:  func() { unilog.Log(ctx, unilog.InfoLevel, "test", "key", "value") },
			wantSkip: 2, // logWithDefault adds 2
		},
		{
			name:     "info helper",
			logFunc:  func() { unilog.Info(ctx, "info test") },
			wantSkip: 2, // logWithDefault adds 2
		},
		{
			name:     "log with skip 1",
			logFunc:  func() { unilog.LogWithSkip(ctx, unilog.WarnLevel, "warn", 1) },
			wantSkip: 3, // 1 + 2 from logWithDefault
		},
		{
			name:     "log with skip 5",
			logFunc:  func() { unilog.LogWithSkip(ctx, unilog.ErrorLevel, "error", 5) },
			wantSkip: 7, // 5 + 2 from logWithDefault
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			skipperLogger.skipCalls = nil
			skipperLogger.buf.Reset()

			tc.logFunc()

			if len(skipperLogger.skipCalls) == 0 {
				t.Fatal("expected skip call to be recorded")
			}

			lastCall := skipperLogger.skipCalls[len(skipperLogger.skipCalls)-1]
			if lastCall.skip != tc.wantSkip {
				t.Errorf("skip value = %d, want %d", lastCall.skip, tc.wantSkip)
			}
		})
	}
}

// TestGlobalLogFunctions covers all global logging functions.
func TestGlobalLogFunctions(t *testing.T) {
	resetDefault()
	defer resetDefault()

	mock := newMockLogger()
	unilog.SetDefault(mock)

	ctx := context.Background()

	tests := []struct {
		name     string
		logFunc  func(context.Context, string, ...any)
		level    unilog.LogLevel
		msg      string
		keyVals  []any
		expected string
	}{
		{
			name:     "trace",
			logFunc:  unilog.Trace,
			level:    unilog.TraceLevel,
			msg:      "trace message",
			keyVals:  []any{"trace", true},
			expected: "TRACE: trace message",
		},
		{
			name:     "debug",
			logFunc:  unilog.Debug,
			level:    unilog.DebugLevel,
			msg:      "debug message",
			keyVals:  nil,
			expected: "DEBUG: debug message",
		},
		{
			name:     "info",
			logFunc:  unilog.Info,
			level:    unilog.InfoLevel,
			msg:      "info message",
			keyVals:  []any{"key", "value"},
			expected: "INFO: info message",
		},
		{
			name:     "warn",
			logFunc:  unilog.Warn,
			level:    unilog.WarnLevel,
			msg:      "warn message",
			keyVals:  []any{"warning", 1},
			expected: "WARN: warn message",
		},
		{
			name:     "error",
			logFunc:  unilog.Error,
			level:    unilog.ErrorLevel,
			msg:      "error message",
			keyVals:  []any{"error", "details"},
			expected: "ERROR: error message",
		},
		{
			name:     "critical",
			logFunc:  unilog.Critical,
			level:    unilog.CriticalLevel,
			msg:      "critical message",
			keyVals:  nil,
			expected: "CRITICAL: critical message",
		},
		{
			name:     "fatal",
			logFunc:  unilog.Fatal,
			level:    unilog.FatalLevel,
			msg:      "fatal message",
			keyVals:  []any{"fatal", true},
			expected: "FATAL: fatal message [FATAL]",
		},
		{
			name:     "panic",
			logFunc:  unilog.Panic,
			level:    unilog.PanicLevel,
			msg:      "panic message",
			keyVals:  []any{"panic", "now"},
			expected: "PANIC: panic message [PANIC]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.buf.Reset()
			tt.logFunc(ctx, tt.msg, tt.keyVals...)

			if got := mock.String(); got != tt.expected {
				t.Errorf("%s() output = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestLogWithNilContext(t *testing.T) {
	resetDefault()
	defer resetDefault()

	mock := newMockLogger()
	unilog.SetDefault(mock)

	// Test with nil context
	tests := []struct {
		name    string
		logFunc func(context.Context, string, ...any)
		msg     string
	}{
		{"info with nil ctx", unilog.Info, "info message"},
		{"error with nil ctx", unilog.Error, "error message"},
		{"debug with nil ctx", unilog.Debug, "debug message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.buf.Reset()

			// Should not panic with nil context
			tt.logFunc(nil, tt.msg)

			if output := mock.String(); !strings.Contains(output, tt.msg) {
				t.Errorf("expected output to contain %q, got %q", tt.msg, output)
			}
		})
	}
}

func TestLogWithKeyValues(t *testing.T) {
	resetDefault()
	defer resetDefault()

	mock := newMockLogger()
	unilog.SetDefault(mock)

	ctx := context.Background()

	tests := []struct {
		name    string
		keyVals []any
		msg     string
	}{
		{
			name:    "empty key values",
			keyVals: []any{},
			msg:     "no attributes",
		},
		{
			name:    "single key-value pair",
			keyVals: []any{"key", "value"},
			msg:     "one attribute",
		},
		{
			name:    "multiple key-value pairs",
			keyVals: []any{"key1", "value1", "key2", 2, "key3", true},
			msg:     "multiple attributes",
		},
		{
			name:    "odd number of key values",
			keyVals: []any{"key1", "value1", "key2"},
			msg:     "odd attributes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.buf.Reset()

			unilog.Info(ctx, tt.msg, tt.keyVals...)

			expected := "INFO: " + tt.msg
			if got := mock.String(); got != expected {
				t.Errorf("Info() with keyValues output = %q, want %q", got, expected)
			}
		})
	}
}
