package zerolog

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/balinomad/go-unilog/handler"
)

// validFormats is the list of supported output formats.
var validFormats = []string{"json", "console"}

// zerologOptions holds configuration for the zerolog logger.
type zerologOptions struct {
	base *handler.BaseOptions
}

// ZerologOption configures zerolog logger creation.
type ZerologOption func(*zerologOptions) error

// WithLevel sets the minimum log level.
func WithLevel(level handler.LogLevel) ZerologOption {
	return func(o *zerologOptions) error {
		return handler.WithLevel(level)(o.base)
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) ZerologOption {
	return func(o *zerologOptions) error {
		return handler.WithOutput(w)(o.base)
	}
}

// WithFormat sets the output format ("json" or "console").
func WithFormat(format string) ZerologOption {
	return func(o *zerologOptions) error {
		return handler.WithFormat(format)(o.base)
	}
}

// WithCaller enables or disables source location reporting.
// If enabled, the handler will include the source location
// of the log call site in the log record.
// This can be useful for debugging, but may incur a performance hit
// due to the additional stack frame analysis. The default value is false.
func WithCaller(enabled bool) ZerologOption {
	return func(o *zerologOptions) error {
		return handler.WithCaller(enabled)(o.base)
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) ZerologOption {
	return func(o *zerologOptions) error {
		return handler.WithTrace(enabled)(o.base)
	}
}

// historyOp is a closure that applies attributes or groups to a zerolog Context.
type historyOp func(zerolog.Context) zerolog.Context

// zerologHandler is a wrapper around zerolog.Logger.
type zerologHandler struct {
	base    *handler.BaseHandler
	logger  zerolog.Logger
	history []historyOp

	// Cached from base for lock-free hot-path
	withCaller bool
	withTrace  bool
}

// Ensure zerologHandler implements the following interfaces.
var (
	_ handler.Handler        = (*zerologHandler)(nil)
	_ handler.Chainer        = (*zerologHandler)(nil)
	_ handler.Configurable   = (*zerologHandler)(nil)
	_ handler.CallerAdjuster = (*zerologHandler)(nil)
	_ handler.FeatureToggler = (*zerologHandler)(nil)
	_ handler.MutableConfig  = (*zerologHandler)(nil)
)

// levelMapper maps unilog log levels to zerolog log levels.
var levelMapper = handler.NewLevelMapper(
	zerolog.TraceLevel, // Trace
	zerolog.DebugLevel, // Debug
	zerolog.InfoLevel,  // Info
	zerolog.WarnLevel,  // Warn
	zerolog.ErrorLevel, // Error
	zerolog.ErrorLevel, // Critical (no native equivalent)
	zerolog.FatalLevel, // Fatal
	zerolog.PanicLevel, // Panic
)

// New creates a new handler.Handler instance backed by zerolog.
func New(opts ...ZerologOption) (handler.Handler, error) {
	o := &zerologOptions{
		base: &handler.BaseOptions{
			Level:        handler.InfoLevel,
			Output:       os.Stderr,
			Format:       "json",
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

	h := &zerologHandler{
		base:       base,
		withCaller: base.CallerEnabled(),
		withTrace:  base.TraceEnabled(),
		history:    make([]historyOp, 0),
	}
	h.rebuildLogger()

	return h, nil
}

// Handle implements the handler.Handler interface for zerolog.
func (h *zerologHandler) Handle(_ context.Context, r *handler.Record) error {
	if !h.Enabled(r.Level) {
		return nil
	}

	// Use cached logger if no dynamic skip is needed
	l := h.logger
	if h.withCaller && r.Skip > 0 {
		// Dynamic skip: lightweight clone of the logger context
		l = l.With().CallerWithSkipFrameCount(h.base.CallerSkip() + r.Skip).Logger()
	}

	// Start event at appropriate level
	event := l.WithLevel(levelMapper.Map(r.Level))
	event.Time(zerolog.TimestampFieldName, r.Time)

	// Add key-value pairs
	for i := 0; i < len(r.KeyValues)-1; i += 2 {
		key := fmt.Sprint(r.KeyValues[i])
		addField(event, key, r.KeyValues[i+1])
	}

	// Add stack trace if enabled
	if h.withTrace && r.Level >= handler.ErrorLevel {
		event.Stack()
	}

	// Send message
	event.Msg(r.Message)

	return nil
}

// Enabled checks if the given log level is enabled.
func (h *zerologHandler) Enabled(level handler.LogLevel) bool {
	return h.base.Enabled(level)
}

// HandlerState returns the underlying BaseHandler.
func (h *zerologHandler) HandlerState() handler.HandlerState {
	return h.base
}

// Features returns the supported HandlerFeatures.
func (h *zerologHandler) Features() handler.HandlerFeatures {
	return handler.NewHandlerFeatures(
		handler.FeatNativeCaller |
			handler.FeatNativeGroup |
			handler.FeatDynamicLevel |
			handler.FeatDynamicOutput |
			handler.FeatZeroAlloc)
}

// WithAttrs returns a new logger with the provided keyValues added to the context.
func (h *zerologHandler) WithAttrs(keyValues []any) handler.Chainer {
	if len(keyValues) < 2 {
		return h
	}

	clone := h.clone()

	// Create closure for replay
	op := func(ctx zerolog.Context) zerolog.Context {
		for i := 0; i < len(keyValues)-1; i += 2 {
			key := fmt.Sprint(keyValues[i])
			ctx = addContextField(ctx, key, keyValues[i+1])
		}
		return ctx
	}

	clone.history = append(clone.history, op)
	clone.logger = op(clone.logger.With()).Logger()

	return clone
}

// WithGroup returns a Logger that starts a group.
func (h *zerologHandler) WithGroup(name string) handler.Chainer {
	if name == "" {
		return h
	}

	clone := h.clone()

	// Create closure for replay
	op := func(ctx zerolog.Context) zerolog.Context {
		return ctx.Str("group", name)
	}

	clone.history = append(clone.history, op)
	clone.logger = op(clone.logger.With()).Logger()

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (h *zerologHandler) SetLevel(level handler.LogLevel) error {
	if err := h.base.SetLevel(level); err != nil {
		return err
	}

	h.logger = h.logger.Level(levelMapper.Map(level))

	return nil
}

// SetOutput sets the log destination.
func (h *zerologHandler) SetOutput(w io.Writer) error {
	return h.base.SetOutput(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (h *zerologHandler) CallerSkip() int {
	return h.base.CallerSkip()
}

// WithCaller returns a new handler with caller reporting enabled or disabled.
func (h *zerologHandler) WithCaller(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithCaller(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithTrace returns a new handler that enables or disables stack trace logging.
func (h *zerologHandler) WithTrace(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithTrace(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithLevel returns a new handler with a new minimum level applied.
func (h *zerologHandler) WithLevel(level handler.LogLevel) handler.Configurable {
	newBase, err := h.base.WithLevel(level)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithOutput returns a new handler with the output writer set permanently.
func (h *zerologHandler) WithOutput(w io.Writer) handler.Configurable {
	newBase, err := h.base.WithOutput(w)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithCallerSkip returns a new handler with the caller skip permanently adjusted.
func (h *zerologHandler) WithCallerSkip(skip int) handler.CallerAdjuster {
	current := h.base.CallerSkip()
	if skip == current {
		return h
	}

	return h.WithCallerSkipDelta(skip - current)
}

// WithCallerSkipDelta returns a new handler with the caller skip altered by delta.
func (h *zerologHandler) WithCallerSkipDelta(delta int) handler.CallerAdjuster {
	if delta == 0 {
		return h
	}

	newBase, err := h.base.WithCallerSkipDelta(delta)
	if err != nil {
		return h
	}

	return h.deepClone(newBase)
}

// rebuildLogger rebuilds the zerolog logger.
func (h *zerologHandler) rebuildLogger() {
	var w io.Writer = h.base.AtomicWriter()
	if h.base.Format() == "console" {
		w = zerolog.ConsoleWriter{
			Out:        h.base.AtomicWriter(),
			TimeFormat: time.RFC3339,
		}
	}

	cx := zerolog.New(w).Level(levelMapper.Map(h.base.Level())).With().Timestamp()

	if h.base.CallerEnabled() {
		cx = cx.CallerWithSkipFrameCount(h.base.CallerSkip())
	}

	for _, op := range h.history {
		cx = op(cx)
	}

	h.logger = cx.Logger()
	h.withCaller = h.base.CallerEnabled()
	h.withTrace = h.base.TraceEnabled()
}

// clone returns a shallow copy of the handler.
func (h *zerologHandler) clone() *zerologHandler {
	return &zerologHandler{
		base:       h.base,
		logger:     h.logger,
		withCaller: h.withCaller,
		withTrace:  h.withTrace,
	}
}

// deepClone returns a deep copy of the handler with a new BaseHandler.
func (h *zerologHandler) deepClone(base *handler.BaseHandler) *zerologHandler {
	clone := h.clone()
	clone.base = base
	clone.rebuildLogger()

	return clone
}

// addField adds a typed field to a zerolog event for optimal performance.
func addField(event *zerolog.Event, key string, val any) {
	switch v := val.(type) {
	case string:
		event.Str(key, v)
	case int:
		event.Int(key, v)
	case error:
		event.Err(v)
	case bool:
		event.Bool(key, v)
	case int64:
		event.Int64(key, v)
	case float64:
		event.Float64(key, v)
	case time.Time:
		event.Time(key, v)
	case time.Duration:
		event.Dur(key, v)
	case uint64:
		event.Uint64(key, v)
	case uint:
		event.Uint(key, v)
	case int8:
		event.Int8(key, v)
	case int16:
		event.Int16(key, v)
	case []byte:
		event.Bytes(key, v)
	default:
		event.Interface(key, v)
	}
}

// addContextField adds a typed field to a zerolog context for optimal performance.
func addContextField(ctx zerolog.Context, key string, val any) zerolog.Context {
	switch v := val.(type) {
	case string:
		return ctx.Str(key, v)
	case int:
		return ctx.Int(key, v)
	case error:
		return ctx.Err(v)
	case bool:
		return ctx.Bool(key, v)
	case int64:
		return ctx.Int64(key, v)
	case float64:
		return ctx.Float64(key, v)
	case time.Time:
		return ctx.Time(key, v)
	case time.Duration:
		return ctx.Dur(key, v)
	case uint64:
		return ctx.Uint64(key, v)
	case uint:
		return ctx.Uint(key, v)
	case int8:
		return ctx.Int8(key, v)
	case int16:
		return ctx.Int16(key, v)
	case []byte:
		return ctx.Bytes(key, v)
	default:
		return ctx.Interface(key, v)
	}
}
