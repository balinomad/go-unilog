package logrus

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/sirupsen/logrus"

	"github.com/balinomad/go-unilog/handler"
)

// validFormats is the list of supported output formats.
var validFormats = []string{"json", "text"}

// logrusOptions holds configuration for the logrus logger.
type logrusOptions struct {
	base *handler.BaseOptions
}

// LogrusOption configures logrus logger creation.
type LogrusOption func(*logrusOptions) error

// WithLevel sets the minimum log level.
func WithLevel(level handler.LogLevel) LogrusOption {
	return func(o *logrusOptions) error {
		return handler.WithLevel(level)(o.base)
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) LogrusOption {
	return func(o *logrusOptions) error {
		return handler.WithOutput(w)(o.base)
	}
}

// WithFormat sets the output format ("json" or "text").
func WithFormat(format string) LogrusOption {
	return func(o *logrusOptions) error {
		return handler.WithFormat(format)(o.base)
	}
}

// WithCaller enables or disables source location reporting.
// If enabled, the handler will include the source location
// of the log call site in the log record.
// This can be useful for debugging, but may incur a performance hit
// due to the additional stack frame analysis. The default value is false.
func WithCaller(enabled bool) LogrusOption {
	return func(o *logrusOptions) error {
		return handler.WithCaller(enabled)(o.base)
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) LogrusOption {
	return func(o *logrusOptions) error {
		return handler.WithTrace(enabled)(o.base)
	}
}

// logrusHandler is a wrapper around logrus.Logger.
type logrusHandler struct {
	base   *handler.BaseHandler
	logger *logrus.Logger
	entry  *logrus.Entry

	// Cached from base for lock-free hot-path
	withCaller bool
	withTrace  bool
}

// Ensure logrusHandler implements the following interfaces.
var (
	_ handler.Handler        = (*logrusHandler)(nil)
	_ handler.Chainer        = (*logrusHandler)(nil)
	_ handler.Configurable   = (*logrusHandler)(nil)
	_ handler.CallerAdjuster = (*logrusHandler)(nil)
	_ handler.FeatureToggler = (*logrusHandler)(nil)
	_ handler.MutableConfig  = (*logrusHandler)(nil)
)

// levelMapper maps unilog log levels to logrus log levels.
var levelMapper = handler.NewLevelMapper(
	logrus.TraceLevel, // Trace
	logrus.DebugLevel, // Debug
	logrus.InfoLevel,  // Info
	logrus.WarnLevel,  // Warn
	logrus.ErrorLevel, // Error
	logrus.ErrorLevel, // Critical (no native equivalent)
	logrus.FatalLevel, // Fatal
	logrus.PanicLevel, // Panic
)

// New creates a new handler.Handler instance backed by logrus.
func New(opts ...LogrusOption) (handler.Handler, error) {
	o := &logrusOptions{
		base: &handler.BaseOptions{
			Level:        handler.InfoLevel,
			Output:       os.Stderr,
			Format:       "text",
			ValidFormats: validFormats,
		},
	}

	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}

	base, err := handler.NewBaseHandler(o.base)
	if err != nil {
		return nil, err
	}

	// Create logrus logger
	logger := logrus.New()
	logger.SetOutput(base.AtomicWriter())
	logger.SetLevel(levelMapper.Map(base.Level()))

	// Set formatter
	if base.Format() == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	}

	// We can't use native logrus caller reporting, because skip frame is not supported
	logger.SetReportCaller(false)

	return &logrusHandler{
		base:       base,
		logger:     logger,
		entry:      logrus.NewEntry(logger),
		withCaller: base.CallerEnabled(),
		withTrace:  base.TraceEnabled(),
	}, nil
}

// Handle implements the handler.Handler interface for logrus.
func (h *logrusHandler) Handle(ctx context.Context, r *handler.Record) error {
	if !h.Enabled(r.Level) {
		return nil
	}

	// Start with entry (may have chained fields)
	entry := h.entry

	// Add context if provided
	if ctx != nil {
		entry = entry.WithContext(ctx)
	}

	// Convert keyValues to logrus.Fields
	n := len(r.KeyValues)
	fields := make(logrus.Fields, n/2+2)
	for i := 0; i < n-1; i += 2 {
		key, ok := r.KeyValues[i].(string)
		if !ok {
			key = fmt.Sprint(r.KeyValues[i])
		}
		fields[key] = r.KeyValues[i+1]
	}

	// Add caller if enabled and not already handled by logger
	if h.withCaller && r.PC != 0 {
		fields["caller"] = resolveFrame(r.PC)
	}

	// Add stack trace if enabled
	if h.withTrace && r.Level >= handler.ErrorLevel {
		fields["stack"] = string(debug.Stack())
	}

	entry.WithFields(fields).Log(levelMapper.Map(r.Level), r.Message)

	return nil
}

// Enabled checks if the given log level is enabled.
func (h *logrusHandler) Enabled(level handler.LogLevel) bool {
	return h.base.Enabled(level)
}

// HandlerState returns the underlying BaseHandler.
func (h *logrusHandler) HandlerState() handler.HandlerState {
	return h.base
}

// Features returns the supported HandlerFeatures.
func (h *logrusHandler) Features() handler.HandlerFeatures {
	return handler.NewHandlerFeatures(
		handler.FeatContextPropagation |
			handler.FeatDynamicLevel |
			handler.FeatDynamicOutput)
}

// WithAttrs returns a new logger with the provided keyValues added to the context.
func (h *logrusHandler) WithAttrs(keyValues []any) handler.Chainer {
	if len(keyValues) < 2 {
		return h
	}

	// Convert keyValues to logrus.Fields
	n := len(keyValues)
	fields := make(logrus.Fields, n/2)
	for i := 0; i < len(keyValues)-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprint(keyValues[i])
		}
		fields[key] = keyValues[i+1]
	}

	clone := h.clone()
	clone.entry = h.entry.WithFields(fields)

	return clone
}

// WithGroup returns a Logger that starts a group (via key prefixing).
func (h *logrusHandler) WithGroup(name string) handler.Chainer {
	if name == "" {
		return h
	}

	base, err := h.base.WithKeyPrefix(name)
	if err != nil {
		return h
	}

	clone := h.clone()
	clone.base = base

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (h *logrusHandler) SetLevel(level handler.LogLevel) error {
	if err := h.base.SetLevel(level); err != nil {
		return err
	}

	h.logger.SetLevel(levelMapper.Map(level))

	return nil
}

// SetOutput sets the log destination.
func (h *logrusHandler) SetOutput(w io.Writer) error {
	if err := h.base.SetOutput(w); err != nil {
		return err
	}

	h.logger.SetOutput(h.base.AtomicWriter())

	return nil
}

// CallerSkip returns the current number of stack frames being skipped.
func (h *logrusHandler) CallerSkip() int {
	return h.base.CallerSkip()
}

// WithCaller returns a new handler with caller reporting enabled or disabled.
func (h *logrusHandler) WithCaller(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithCaller(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithTrace returns a new handler that enables or disables stack trace logging.
func (h *logrusHandler) WithTrace(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithTrace(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithLevel returns a new handler with a new minimum level applied.
func (h *logrusHandler) WithLevel(level handler.LogLevel) handler.Configurable {
	newBase, err := h.base.WithLevel(level)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithOutput returns a new handler with the output writer set permanently.
func (h *logrusHandler) WithOutput(w io.Writer) handler.Configurable {
	newBase, err := h.base.WithOutput(w)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithCallerSkip returns a new handler with the caller skip permanently adjusted.
func (h *logrusHandler) WithCallerSkip(skip int) handler.CallerAdjuster {
	current := h.base.CallerSkip()
	if skip == current {
		return h
	}

	return h.WithCallerSkipDelta(skip - current)
}

// WithCallerSkipDelta returns a new handler with the caller skip altered by delta.
func (h *logrusHandler) WithCallerSkipDelta(delta int) handler.CallerAdjuster {
	if delta == 0 {
		return h
	}

	newBase, err := h.base.WithCallerSkipDelta(delta)
	if err != nil {
		return h
	}

	return h.deepClone(newBase)
}

// clone returns a shallow copy of the handler.
func (h *logrusHandler) clone() *logrusHandler {
	return &logrusHandler{
		base:       h.base,
		logger:     h.logger,
		entry:      h.entry,
		withCaller: h.withCaller,
		withTrace:  h.withTrace,
	}
}

// deepClone returns a deep copy of the handler with a new BaseHandler.
func (h *logrusHandler) deepClone(base *handler.BaseHandler) *logrusHandler {
	logger := logrus.New()
	logger.SetOutput(base.AtomicWriter())
	logger.SetLevel(levelMapper.Map(base.Level()))

	if base.Format() == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	}

	return &logrusHandler{
		base:       base,
		logger:     logger,
		entry:      logrus.NewEntry(logger).WithFields(h.entry.Data),
		withCaller: base.CallerEnabled(),
		withTrace:  base.TraceEnabled(),
	}
}

// resolveFrame converts a PC to a source location string.
func resolveFrame(pc uintptr) string {
	frames := runtime.CallersFrames([]uintptr{pc})
	frame, _ := frames.Next()
	return fmt.Sprintf("%s:%d", frame.File, frame.Line)
}
