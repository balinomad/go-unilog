package unilog_test

import (
	"bytes"
	"context"
	"sync"

	"github.com/balinomad/go-unilog"
)

// mockLogger is a simple test logger implementation.
// Fatal and Panic are implemented without exiting the process.
type mockLogger struct {
	mu         sync.Mutex
	buf        *bytes.Buffer
	callerSkip int
}

// Ensure mockLogger implements the Logger interface
var _ unilog.Logger = (*mockLogger)(nil)

// newMockLogger returns a new mockLogger.
// The returned logger can be used for testing, and its log output can be
// inspected via the Buffer field.
func newMockLogger() *mockLogger {
	return &mockLogger{
		buf: &bytes.Buffer{},
	}
}

// Log logs a message at the given level.
func (m *mockLogger) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.WriteString(level.String() + ": " + msg)
	if level == unilog.FatalLevel {
		// Simulate fatal behavior without actually exiting
		m.buf.WriteString(" [FATAL]")
	}
	if level == unilog.PanicLevel {
		// Simulate panic behavior without actually panicking
		m.buf.WriteString(" [PANIC]")
	}
}

// Enabled returns true for all log levels.
func (m *mockLogger) Enabled(level unilog.LogLevel) bool {
	return true
}

// With returns the logger unchanged.
func (m *mockLogger) With(keyValues ...any) unilog.Logger {
	return m
}

// WithGroup returns the logger unchanged.
func (m *mockLogger) WithGroup(name string) unilog.Logger {
	return m
}

// Trace is a convenience method for logging at the trace level.
func (m *mockLogger) Trace(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.TraceLevel, msg, keyValues...)
}

// Debug is a convenience method for logging at the debug level.
func (m *mockLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.DebugLevel, msg, keyValues...)
}

// Info is a convenience method for logging at the info level.
func (m *mockLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.InfoLevel, msg, keyValues...)
}

// Warn is a convenience method for logging at the warn level.
func (m *mockLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.WarnLevel, msg, keyValues...)
}

// Error is a convenience method for logging at the error level.
func (m *mockLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.ErrorLevel, msg, keyValues...)
}

// Critical is a convenience method for logging at the critical level.
func (m *mockLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.CriticalLevel, msg, keyValues...)
}

// Fatal is a convenience method for logging at the fatal level.
func (m *mockLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.FatalLevel, msg, keyValues...)
}

// Panic is a convenience method for logging at the panic level.
func (m *mockLogger) Panic(ctx context.Context, msg string, keyValues ...any) {
	m.Log(ctx, unilog.PanicLevel, msg, keyValues...)
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
	_ unilog.Logger        = (*mockLoggerWithSkipper)(nil)
	_ unilog.CallerSkipper = (*mockLoggerWithSkipper)(nil)
)

// skipCall holds the parameters for a LogWithSkip call.
type skipCall struct {
	level     unilog.LogLevel
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
func (m *mockLoggerWithSkipper) LogWithSkip(ctx context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
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
func (m *mockLoggerWithSkipper) WithCallerSkip(skip int) (unilog.Logger, error) {
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
func (m *mockLoggerWithSkipper) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	return m.WithCallerSkip(m.callerSkip + delta)
}

// resetDefault resets the global state for tests.
// TODO: This must be fixed.
func resetDefault() {
	unilog.SetDefault(nil)
	_, _ = unilog.Default(), unilog.LoggerFromContextOrDefault(context.Background())
}
