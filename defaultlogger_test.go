package unilog

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
)

// mockLogger is a simple mock implementation of the Logger interface for testing.
// Fatal and Panic are implemented without exiting the process.
type mockLogger struct {
	mu         sync.Mutex
	buf        *bytes.Buffer
	callerSkip int
}

// Ensure mockLogger implements the Logger interface
var _ Logger = (*mockLogger)(nil)

// newMockLogger returns a new mockLogger.
// The returned logger can be used for testing, and its log output can be
// inspected via the Buffer field.
func newMockLogger() *mockLogger {
	return &mockLogger{
		buf: &bytes.Buffer{},
	}
}

// Log logs a message at the given level.
func (m *mockLogger) Log(_ context.Context, level LogLevel, msg string, keyValues ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.WriteString(level.String() + ": " + msg)
	if level == FatalLevel {
		// Simulate fatal behavior without actually exiting
		m.buf.WriteString(" [FATAL]")
	}
	if level == PanicLevel {
		// Simulate panic behavior without actually panicking
		m.buf.WriteString(" [PANIC]")
	}
}

// Enabled returns true for all log levels.
func (m *mockLogger) Enabled(level LogLevel) bool {
	return true
}

// With returns the logger unchanged.
func (m *mockLogger) With(keyValues ...any) Logger {
	return m
}

// WithGroup returns the logger unchanged.
func (m *mockLogger) WithGroup(name string) Logger {
	return m
}

// Trace is a convenience method for logging at the trace level.
func (m *mockLogger) Trace(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, TraceLevel, msg, keyValues...)
}

// Debug is a convenience method for logging at the debug level.
func (m *mockLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, DebugLevel, msg, keyValues...)
}

// Info is a convenience method for logging at the info level.
func (m *mockLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, InfoLevel, msg, keyValues...)
}

// Warn is a convenience method for logging at the warn level.
func (m *mockLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, WarnLevel, msg, keyValues...)
}

// Error is a convenience method for logging at the error level.
func (m *mockLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, ErrorLevel, msg, keyValues...)
}

// Critical is a convenience method for logging at the critical level.
func (m *mockLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, CriticalLevel, msg, keyValues...)
}

// Fatal is a convenience method for logging at the fatal level.
func (m *mockLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, FatalLevel, msg, keyValues...)
}

// Panic is a convenience method for logging at the panic level.
func (m *mockLogger) Panic(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, PanicLevel, msg, keyValues...)
}

// String returns the log output as a string.
func (m *mockLogger) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

// mockLoggerWithSkipper implements both Logger and CallerSkipper interfaces.
type mockLoggerWithSkipper struct {
	*mockLogger
	skipCalls []skipCall
}

// Ensure mockLogger implements the following interfaces
var (
	_ Logger        = (*mockLoggerWithSkipper)(nil)
	_ CallerSkipper = (*mockLoggerWithSkipper)(nil)
)

// skipCall holds the parameters for a LogWithSkip call.
type skipCall struct {
	level     LogLevel
	msg       string
	skip      int
	keyValues []any
}

// newMockLoggerWithSkipper returns a new mockLoggerWithSkipper.
func newMockLoggerWithSkipper() *mockLoggerWithSkipper {
	return &mockLoggerWithSkipper{
		mockLogger: newMockLogger(),
		skipCalls:  []skipCall{},
	}
}

// LogWithSkip logs a message at the given level, skipping the given number of stack frames.
func (m *mockLoggerWithSkipper) LogWithSkip(ctx context.Context, level LogLevel, msg string, skip int, keyValues ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skipCalls = append(m.skipCalls, skipCall{
		level:     level,
		msg:       msg,
		skip:      skip,
		keyValues: keyValues,
	})
	m.buf.WriteString(level.String() + ": " + msg + " [skip:" + string(rune(skip+'0')) + "]")
}

// CallerSkip returns the number of stack frames to skip.
func (m *mockLoggerWithSkipper) CallerSkip() int {
	return m.callerSkip
}

// WithCallerSkip returns a new Logger with the caller skip set.
func (m *mockLoggerWithSkipper) WithCallerSkip(skip int) (Logger, error) {
	newLogger := &mockLoggerWithSkipper{
		mockLogger: &mockLogger{
			buf:        m.buf,
			callerSkip: skip,
		},
		skipCalls: m.skipCalls,
	}
	return newLogger, nil
}

// WithCallerSkipDelta returns a new Logger with caller skip adjusted by delta.
func (m *mockLoggerWithSkipper) WithCallerSkipDelta(delta int) (Logger, error) {
	return m.WithCallerSkip(m.callerSkip + delta)
}

// resetDefault resets the global state for tests.
func resetDefault() {
	defaultLogger = nil
	once = sync.Once{}
}

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
				SetDefault(newMockLogger())
			},
			wantType: "*unilog.mockLogger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetDefault()
			defer resetDefault()

			tt.setup()

			logger := Default()
			if logger == nil {
				t.Fatal("Default() returned nil")
			}

			// Check type
			switch tt.wantType {
			case "*unilog.fallbackLogger":
				if _, ok := logger.(*fallbackLogger); !ok {
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
	loggers := make([]Logger, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			loggers[idx] = Default()
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
		logger Logger
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
				SetDefault(tt.logger)
				got := Default()
				if got != tt.logger {
					t.Errorf("Default() = %v, want %v", got, tt.logger)
				}
			} else {
				// Setting nil should cause Default() to create fallback
				SetDefault(nil)
				got := Default()
				if _, ok := got.(*fallbackLogger); !ok {
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
	SetDefault(mock)

	ctx := context.Background()

	tests := []struct {
		name     string
		level    LogLevel
		msg      string
		keyVals  []any
		expected string
	}{
		{
			name:     "log trace message",
			level:    TraceLevel,
			msg:      "trace info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "TRACE: trace info",
		},
		{
			name:     "log debug message",
			level:    DebugLevel,
			msg:      "debug info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "DEBUG: debug info",
		},
		{
			name:     "log info message",
			level:    InfoLevel,
			msg:      "test message",
			keyVals:  []any{"key", "value"},
			expected: "INFO: test message",
		},
		{
			name:     "log warn message",
			level:    WarnLevel,
			msg:      "warn info",
			keyVals:  nil,
			expected: "WARN: warn info",
		},
		{
			name:     "log error message",
			level:    ErrorLevel,
			msg:      "error occurred",
			keyVals:  nil,
			expected: "ERROR: error occurred",
		},
		{
			name:     "log critical message",
			level:    CriticalLevel,
			msg:      "critical info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "CRITICAL: critical info",
		},
		{
			name:     "log fatal message",
			level:    FatalLevel,
			msg:      "fatal info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "FATAL: fatal info [FATAL]",
		},
		{
			name:     "log panic message",
			level:    PanicLevel,
			msg:      "panic info",
			keyVals:  []any{"id", 123, "name", "test"},
			expected: "PANIC: panic info [PANIC]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.buf.Reset()
			Log(ctx, tt.level, tt.msg, tt.keyVals...)

			if got := mock.String(); got != tt.expected {
				t.Errorf("Log() output = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLogWithSkip(t *testing.T) {
	tests := []struct {
		name         string
		logger       Logger
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

			SetDefault(tt.logger)
			ctx := context.Background()

			LogWithSkip(ctx, InfoLevel, tt.msg, tt.skip)

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
	SetDefault(skipperLogger)

	ctx := context.Background()

	// Test various skip values
	testCases := []struct {
		name     string
		logFunc  func()
		wantSkip int
	}{
		{
			name:     "direct log call",
			logFunc:  func() { Log(ctx, InfoLevel, "test", "key", "value") },
			wantSkip: 2, // logWithDefault adds 2
		},
		{
			name:     "info helper",
			logFunc:  func() { Info(ctx, "info test") },
			wantSkip: 2, // logWithDefault adds 2
		},
		{
			name:     "log with skip 1",
			logFunc:  func() { LogWithSkip(ctx, WarnLevel, "warn", 1) },
			wantSkip: 3, // 1 + 2 from logWithDefault
		},
		{
			name:     "log with skip 5",
			logFunc:  func() { LogWithSkip(ctx, ErrorLevel, "error", 5) },
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
	SetDefault(mock)

	ctx := context.Background()

	tests := []struct {
		name     string
		logFunc  func(context.Context, string, ...any)
		level    LogLevel
		msg      string
		keyVals  []any
		expected string
	}{
		{
			name:     "trace",
			logFunc:  Trace,
			level:    TraceLevel,
			msg:      "trace message",
			keyVals:  []any{"trace", true},
			expected: "TRACE: trace message",
		},
		{
			name:     "debug",
			logFunc:  Debug,
			level:    DebugLevel,
			msg:      "debug message",
			keyVals:  nil,
			expected: "DEBUG: debug message",
		},
		{
			name:     "info",
			logFunc:  Info,
			level:    InfoLevel,
			msg:      "info message",
			keyVals:  []any{"key", "value"},
			expected: "INFO: info message",
		},
		{
			name:     "warn",
			logFunc:  Warn,
			level:    WarnLevel,
			msg:      "warn message",
			keyVals:  []any{"warning", 1},
			expected: "WARN: warn message",
		},
		{
			name:     "error",
			logFunc:  Error,
			level:    ErrorLevel,
			msg:      "error message",
			keyVals:  []any{"error", "details"},
			expected: "ERROR: error message",
		},
		{
			name:     "critical",
			logFunc:  Critical,
			level:    CriticalLevel,
			msg:      "critical message",
			keyVals:  nil,
			expected: "CRITICAL: critical message",
		},
		{
			name:     "fatal",
			logFunc:  Fatal,
			level:    FatalLevel,
			msg:      "fatal message",
			keyVals:  []any{"fatal", true},
			expected: "FATAL: fatal message [FATAL]",
		},
		{
			name:     "panic",
			logFunc:  Panic,
			level:    PanicLevel,
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
	SetDefault(mock)

	// Test with nil context
	tests := []struct {
		name    string
		logFunc func(context.Context, string, ...any)
		msg     string
	}{
		{"info with nil ctx", Info, "info message"},
		{"error with nil ctx", Error, "error message"},
		{"debug with nil ctx", Debug, "debug message"},
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
	SetDefault(mock)

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

			Info(ctx, tt.msg, tt.keyVals...)

			expected := "INFO: " + tt.msg
			if got := mock.String(); got != expected {
				t.Errorf("Info() with keyValues output = %q, want %q", got, expected)
			}
		})
	}
}
