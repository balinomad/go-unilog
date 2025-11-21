package unilog_test

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/balinomad/go-unilog"
	"github.com/balinomad/go-unilog/handler"
)

// mockHandlerState is a helper for testing specific handler states.
type mockHandlerState struct{ caller bool }

var _ handler.HandlerState = (*mockHandlerState)(nil)

func (s *mockHandlerState) CallerEnabled() bool { return s.caller }
func (s *mockHandlerState) TraceEnabled() bool  { return false }
func (s *mockHandlerState) CallerSkip() int     { return 0 }

// mockFullHandler implements all optional handler interfaces to test delegation.
// It is thread-safe to support concurrent testing.
type mockFullHandler struct {
	mu sync.Mutex // Added for concurrency safety

	// State verification
	callCount  int
	lastRecord *handler.Record
	lastOp     string
	lastVal    any
	history    []string // Trace of operations

	// Configuration (preserved on clone)
	enabled    bool
	state      handler.HandlerState
	features   handler.HandlerFeatures
	errHandle  error
	errMutable error
	errSync    error
}

// Ensure mockFullHandler implements all interfaces
var (
	_ handler.Handler        = (*mockFullHandler)(nil)
	_ handler.CallerAdjuster = (*mockFullHandler)(nil)
	_ handler.FeatureToggler = (*mockFullHandler)(nil)
	_ handler.Configurable   = (*mockFullHandler)(nil)
	_ handler.MutableConfig  = (*mockFullHandler)(nil)
	_ handler.Syncer         = (*mockFullHandler)(nil)
	_ handler.Chainer        = (*mockFullHandler)(nil)
)

func newMockHandler() *mockFullHandler {
	return &mockFullHandler{
		enabled:  true,
		state:    &mockHandlerState{},
		features: handler.NewHandlerFeatures(0),
		history:  []string{},
	}
}

// clone creates a safe copy of configuration fields without copying the mutex.
// Used by With* methods to simulate immutability.
func (h *mockFullHandler) clone() *mockFullHandler {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Deep copy history
	hist := make([]string, len(h.history))
	copy(hist, h.history)

	return &mockFullHandler{
		// Copy configuration
		enabled:    h.enabled,
		state:      h.state,
		features:   h.features,
		errHandle:  h.errHandle,
		errMutable: h.errMutable,
		errSync:    h.errSync,
		history:    hist,
		callCount:  0, // Zero out verification fields for the new instance
	}
}

// recordOp helper to safely record an operation
func (h *mockFullHandler) recordOp(op string, val any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastOp = op
	h.lastVal = val
	h.history = append(h.history, op)
}

// Handle captures the record by value to ensure safety against sync.Pool recycling.
func (h *mockFullHandler) Handle(_ context.Context, r *handler.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.callCount++

	// DEEP COPY: Create a new Record instance and copy fields.
	// Since r is pooled, we cannot keep the pointer 'r'.
	recCopy := *r

	// Copy slice to prevent shared backing array issues if needed,
	// though strictly the logger doesn't mutate the backing array,
	// it's safer for tests to own their data.
	if r.KeyValues != nil {
		recCopy.KeyValues = make([]any, len(r.KeyValues))
		copy(recCopy.KeyValues, r.KeyValues)
	}

	h.lastRecord = &recCopy
	return h.errHandle
}

func (h *mockFullHandler) Enabled(_ unilog.LogLevel) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.enabled
}

func (h *mockFullHandler) HandlerState() handler.HandlerState {
	return h.state
}

func (h *mockFullHandler) Features() handler.HandlerFeatures {
	return h.features
}

// --- Verification Helpers ---

// CallCount is a helper to safely get call count.
func (h *mockFullHandler) CallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.callCount
}

// LastRecord is a helper to safely get last record.
func (h *mockFullHandler) LastRecord() *handler.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.lastRecord
}

// LastOp is a helper to safely get last operation.
func (h *mockFullHandler) LastOp() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.lastOp
}

// LastVal is a helper to safely get last value.
func (h *mockFullHandler) LastVal() any {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.lastVal
}

// History is a helper to safely get history.
func (m *mockFullHandler) History() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return copy to avoid race
	res := make([]string, len(m.history))
	copy(res, m.history)

	return res
}

// --- Chainer ---

func (h *mockFullHandler) WithAttrs(attrs []any) handler.Chainer {
	c := h.clone()
	c.recordOp("WithAttrs", attrs)

	return c
}

func (h *mockFullHandler) WithGroup(name string) handler.Chainer {
	c := h.clone()
	c.recordOp("WithGroup", name)

	return c
}

// --- CallerAdjuster ---

func (h *mockFullHandler) WithCallerSkip(skip int) handler.CallerAdjuster {
	c := h.clone()
	c.recordOp("WithCallerSkip", skip)

	return c
}

func (h *mockFullHandler) WithCallerSkipDelta(delta int) handler.CallerAdjuster {
	c := h.clone()
	c.recordOp("WithCallerSkipDelta", delta)

	return c
}

// --- FeatureToggler ---

func (h *mockFullHandler) WithCaller(enabled bool) handler.FeatureToggler {
	c := h.clone()
	c.recordOp("WithCaller", enabled)

	return c
}

func (h *mockFullHandler) WithTrace(enabled bool) handler.FeatureToggler {
	c := h.clone()
	c.recordOp("WithTrace", enabled)

	return c
}

// --- Configurable ---

func (h *mockFullHandler) WithLevel(level unilog.LogLevel) handler.Configurable {
	c := h.clone()
	c.recordOp("WithLevel", level)

	return c
}

func (h *mockFullHandler) WithOutput(w io.Writer) handler.Configurable {
	c := h.clone()
	c.recordOp("WithOutput", w)

	return c
}

// --- MutableConfig ---

func (h *mockFullHandler) SetLevel(level unilog.LogLevel) error {
	h.recordOp("SetLevel", level)

	return h.errMutable
}

func (h *mockFullHandler) SetOutput(w io.Writer) error {
	h.recordOp("SetOutput", w)

	return h.errMutable
}

// --- Syncer ---
func (h *mockFullHandler) Sync() error {
	h.recordOp("Sync", nil)

	return h.errSync
}

// mockMinimalHandler implements ONLY the core Handler interface.
// It is used to test that the Logger gracefully handles handlers that
// do NOT implement optional interfaces (Chainer, Configurable, etc.).
type mockMinimalHandler struct {
	*mockFullHandler
}

// Ensure mockMinimalHandler implements ONLY Handler
var _ handler.Handler = (*mockMinimalHandler)(nil)

// Explicitly implementing these to satisfy Handler, but nothing else.
// Note: The embedded *mockFullHandler provides implementations for other interfaces,
// but since mockMinimalHandler is a distinct type, type assertions for
// handler.Chainer etc. on a *mockMinimalHandler will fail unless we explicitly
// verify/hide them.
//
// However, in Go, method promotion means *mockMinimalHandler DOES implement
// those interfaces if the embedded field does. To truly hide them, we must
// embed a struct that only has the data, or use interface composition in a way
// that hides methods.
//
// A cleaner approach for "Minimal" is to not embed mockFullHandler directly
// but wrap it.
type mockMinimalWrapper struct {
	target *mockFullHandler
}

func newMockMinimalHandler() *mockMinimalWrapper {
	return &mockMinimalWrapper{target: newMockHandler()}
}

func (m *mockMinimalWrapper) Handle(ctx context.Context, r *handler.Record) error {
	return m.target.Handle(ctx, r)
}
func (m *mockMinimalWrapper) Enabled(l unilog.LogLevel) bool {
	return m.target.Enabled(l)
}
func (m *mockMinimalWrapper) HandlerState() handler.HandlerState {
	return m.target.HandlerState()
}
func (m *mockMinimalWrapper) Features() handler.HandlerFeatures {
	return m.target.Features()
}

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
func (l *mockLogger) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf.WriteString(level.String() + ": " + msg)
	if level == unilog.FatalLevel {
		// Simulate fatal behavior without actually exiting
		l.buf.WriteString(" [FATAL]")
	}
	if level == unilog.PanicLevel {
		// Simulate panic behavior without actually panicking
		l.buf.WriteString(" [PANIC]")
	}
}

// Enabled returns true for all log levels.
func (l *mockLogger) Enabled(level unilog.LogLevel) bool {
	return true
}

// With returns the logger unchanged.
func (l *mockLogger) With(keyValues ...any) unilog.Logger {
	return l
}

// WithGroup returns the logger unchanged.
func (l *mockLogger) WithGroup(name string) unilog.Logger {
	return l
}

// Trace is a convenience method for logging at the trace level.
func (l *mockLogger) Trace(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.TraceLevel, msg, keyValues...)
}

// Debug is a convenience method for logging at the debug level.
func (l *mockLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.DebugLevel, msg, keyValues...)
}

// Info is a convenience method for logging at the info level.
func (l *mockLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.InfoLevel, msg, keyValues...)
}

// Warn is a convenience method for logging at the warn level.
func (l *mockLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.WarnLevel, msg, keyValues...)
}

// Error is a convenience method for logging at the error level.
func (l *mockLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.ErrorLevel, msg, keyValues...)
}

// Critical is a convenience method for logging at the critical level.
func (l *mockLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.CriticalLevel, msg, keyValues...)
}

// Fatal is a convenience method for logging at the fatal level.
func (l *mockLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.FatalLevel, msg, keyValues...)
}

// Panic is a convenience method for logging at the panic level.
func (l *mockLogger) Panic(ctx context.Context, msg string, keyValues ...any) {
	l.Log(ctx, unilog.PanicLevel, msg, keyValues...)
}

// String returns the log output as a string.
func (l *mockLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.buf.String()
}

// mockAdvancedLogger implements both AdvancedLogger interfaces.
type mockAdvancedLogger struct {
	*mockLogger
	skipCalls []skipCall
}

// Ensure mockLogger implements the following interfaces
var (
	_ unilog.AdvancedLogger = (*mockAdvancedLogger)(nil)
)

// skipCall holds the parameters for a LogWithSkip call.
type skipCall struct {
	level     unilog.LogLevel
	msg       string
	skip      int
	keyValues []any
}

// newMockLoggerWithSkipper returns a new mockAdvancedLogger.
func newMockLoggerWithSkipper() *mockAdvancedLogger {
	return &mockAdvancedLogger{
		mockLogger: newMockLogger(),
		skipCalls:  []skipCall{},
	}
}

// LogWithSkip logs a message at the given level, skipping the given number of stack frames.
func (l *mockAdvancedLogger) LogWithSkip(ctx context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.skipCalls = append(l.skipCalls, skipCall{
		level:     level,
		msg:       msg,
		skip:      skip,
		keyValues: keyValues,
	})
	l.buf.WriteString(level.String() + ": " + msg + " [skip:" + string(rune(skip+'0')) + "]")
}

// CallerSkip returns the number of stack frames to skip.
func (l *mockAdvancedLogger) CallerSkip() int {
	return l.callerSkip
}

// WithCallerSkip returns a new Logger with the caller skip set.
func (l *mockAdvancedLogger) WithCallerSkip(skip int) unilog.AdvancedLogger {
	newLogger := &mockAdvancedLogger{
		mockLogger: &mockLogger{
			buf:        l.buf,
			callerSkip: skip,
		},
		skipCalls: l.skipCalls,
	}
	return newLogger
}

// WithCallerSkipDelta returns a new Logger with caller skip adjusted by delta.
func (l *mockAdvancedLogger) WithCallerSkipDelta(delta int) unilog.AdvancedLogger {
	return l.WithCallerSkip(l.callerSkip + delta)
}

// WithCaller returns a new Logger that enables or disables caller resolution.
func (l *mockAdvancedLogger) WithCaller(enabled bool) unilog.AdvancedLogger {
	return l
}

// WithTrace returns a new Logger that enables or disables trace logging.
func (l *mockAdvancedLogger) WithTrace(enabled bool) unilog.AdvancedLogger {
	return l
}

// WithLevel returns a new Logger with a new minimum level applied to the handler.
func (l *mockAdvancedLogger) WithLevel(level unilog.LogLevel) unilog.AdvancedLogger {
	return l
}

// WithOutput returns a new Logger with the output writer set.
func (l *mockAdvancedLogger) WithOutput(w io.Writer) unilog.AdvancedLogger {
	return l
}

// Sync is a no-op for mockAdvancedLogger.
func (l *mockAdvancedLogger) Sync() error {
	return nil
}

// resetDefault resets the global state for tests.
// TODO: This must be fixed.
func resetDefault() {
	unilog.SetDefault(nil)
	_, _ = unilog.Default(), unilog.LoggerFromContextOrDefault(context.Background())
}
