package unilog

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

// mockLogger is a simple mock implementation of the Logger interface for testing.
// Fatal and Panic are implemented without exiting the process.
type mockLogger struct {
	mu      sync.Mutex
	buf     *bytes.Buffer
	enabled bool
}

func newMockLogger(enabled bool) *mockLogger {
	return &mockLogger{buf: &bytes.Buffer{}, enabled: enabled}
}

func (m *mockLogger) Log(_ context.Context, level LogLevel, msg string, keyValues ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.WriteString(level.String() + ": " + msg)
}

func (m *mockLogger) Enabled(level LogLevel) bool {
	return m.enabled
}

func (m *mockLogger) With(keyValues ...any) Logger {
	return m
}

func (m *mockLogger) WithGroup(name string) Logger {
	return m
}

func (m *mockLogger) Trace(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, TraceLevel, msg, keyValues...)
}

func (m *mockLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, DebugLevel, msg, keyValues...)
}
func (m *mockLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, InfoLevel, msg, keyValues...)
}
func (m *mockLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, WarnLevel, msg, keyValues...)
}
func (m *mockLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, ErrorLevel, msg, keyValues...)
}
func (m *mockLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, CriticalLevel, msg, keyValues...)
}
func (m *mockLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, FatalLevel, msg, keyValues...)
}

func (m *mockLogger) Panic(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, FatalLevel, msg, keyValues...)
}

func (m *mockLogger) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

// resetDefault resets the global state for tests.
func resetDefault() {
	defaultLogger = nil
	once = sync.Once{}
}

func TestDefault(t *testing.T) {
	resetDefault()
	defer resetDefault()

	t.Run("Initial call creates fallback", func(t *testing.T) {
		logger := Default()
		if logger == nil {
			t.Fatal("Default() returned nil")
		}
		if _, ok := logger.(*fallbackLogger); !ok {
			t.Errorf("Default() did not return a *fallbackLogger, got %T", logger)
		}
	})

	t.Run("Subsequent calls return same instance", func(t *testing.T) {
		logger1 := Default()
		logger2 := Default()
		if logger1 != logger2 {
			t.Error("Default() returned a different instance on subsequent call")
		}
	})
}

func TestSetDefault(t *testing.T) {
	resetDefault()
	defer resetDefault()

	mock := newMockLogger(true)
	SetDefault(mock)

	logger := Default()
	if logger != mock {
		t.Error("Default() did not return the logger set by SetDefault")
	}
}

func TestGlobalLogFunctions(t *testing.T) {
	resetDefault()
	defer resetDefault()

	mock := newMockLogger(true)
	SetDefault(mock)

	ctx := context.Background()

	tests := []struct {
		name    string
		logFunc func(ctx context.Context, msg string, keyValues ...any)
		level   LogLevel
		msg     string
	}{
		{"Trace", Trace, TraceLevel, "trace message"},
		{"Debug", Debug, DebugLevel, "debug message"},
		{"Info", Info, InfoLevel, "info message"},
		{"Warn", Warn, WarnLevel, "warn message"},
		{"Error", Error, ErrorLevel, "error message"},
		{"Critical", Critical, CriticalLevel, "critical message"},
		{"Fatal", Fatal, FatalLevel, "fatal message"},
		{"Panic", Panic, PanicLevel, "panic message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.buf.Reset()
			tt.logFunc(ctx, tt.msg)

			expected := tt.level.String() + ": " + tt.msg
			if got := mock.String(); got != expected {
				t.Errorf("Global %s() log output = %q, want %q", tt.name, got, expected)
			}
		})
	}
}
