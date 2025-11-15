package slog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/balinomad/go-unilog/handler"
)

// validFormats is the list of supported output formats.
var validFormats = []string{"json", "text"}

// slogOptions holds configuration for the slog logger.
type slogOptions struct {
	base        *handler.BaseOptions
	replaceAttr func([]string, slog.Attr) slog.Attr // slog-specific option
}

// SlogOption configures slog logger creation.
type SlogOption func(*slogOptions) error

// WithLevel sets the minimum log level.
func WithLevel(level handler.LogLevel) SlogOption {
	return func(o *slogOptions) error {
		return handler.WithLevel(level)(o.base)
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) SlogOption {
	return func(o *slogOptions) error {
		return handler.WithOutput(w)(o.base)
	}
}

// WithFormat sets the output format ("json" or "text").
func WithFormat(format string) SlogOption {
	return func(o *slogOptions) error {
		return handler.WithFormat(format)(o.base)
	}
}

// WithCaller enables source injection. Optional skip overrides the user skip frames.
func WithCaller(enabled bool, skip ...int) SlogOption {
	return func(o *slogOptions) error {
		return handler.WithCaller(enabled)(o.base)
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) SlogOption {
	return func(o *slogOptions) error {
		return handler.WithTrace(enabled)(o.base)
	}
}

// WithReplaceAttr sets a custom attribute transformation function.
func WithReplaceAttr(fn func([]string, slog.Attr) slog.Attr) SlogOption {
	return func(o *slogOptions) error {
		o.replaceAttr = fn
		return nil
	}
}

// slogHandler is a wrapper around Go's [log/slog] logger.
type slogHandler struct {
	base        *handler.BaseHandler
	logger      *slog.Logger
	level       *slog.LevelVar
	handler     slog.Handler
	replaceAttr func([]string, slog.Attr) slog.Attr

	// Cached from base for lock-free hot-path
	withCaller bool
	withTrace  bool
}

// Ensure slogLogger implements the following interfaces.
var (
	_ handler.Handler         = (*slogHandler)(nil)
	_ handler.Chainer         = (*slogHandler)(nil)
	_ handler.AdvancedHandler = (*slogHandler)(nil)
	_ handler.Configurator    = (*slogHandler)(nil)
)

// levelMapper maps unilog log levels to slog log levels.
var levelMapper = handler.NewLevelMapper(
	slog.Level(-8),  // Trace
	slog.LevelDebug, // Debug
	slog.LevelInfo,  // Info
	slog.LevelWarn,  // Warn
	slog.LevelError, // Error
	slog.Level(12),  // Critical
	slog.Level(16),  // Fatal
	slog.Level(20),  // Panic
)

// New creates a new handler.Handler instance backed by [log/slog].
func New(opts ...SlogOption) (handler.Handler, error) {
	o := &slogOptions{
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

	levelVar := new(slog.LevelVar)
	levelVar.Set(levelMapper.Map(base.Level()))

	handlerOpts := &slog.HandlerOptions{
		Level:       levelVar,
		AddSource:   base.CallerEnabled(),
		ReplaceAttr: o.replaceAttr,
	}

	var h slog.Handler
	if base.Format() == "text" {
		h = slog.NewTextHandler(base.AtomicWriter(), handlerOpts)
	} else {
		h = slog.NewJSONHandler(base.AtomicWriter(), handlerOpts)
	}

	return &slogHandler{
		base:        base,
		logger:      slog.New(h),
		level:       levelVar,
		handler:     h,
		replaceAttr: o.replaceAttr,
		withCaller:  base.CallerEnabled(),
		withTrace:   base.TraceEnabled(),
	}, nil
}

// Handle implements the handler.Handler interface for slog.
func (h *slogHandler) Handle(ctx context.Context, r *handler.Record) error {
	if !h.Enabled(r.Level) {
		return nil
	}

	// Convert keyValues to slog.Attr slice
	attrs := keyValuesToSlogAttrs(r.KeyValues)

	// Only add stack if enabled and error-level
	if h.withTrace && r.Level >= handler.ErrorLevel {
		attrs = append(attrs, slog.String("stack", string(debug.Stack())))
	}

	// Build slog.Record with PC for native caller support
	rec := slog.NewRecord(r.Time, levelMapper.Map(r.Level), r.Message, r.PC)
	rec.AddAttrs(attrs...)

	// Use ctx for context propagation
	h.logger.Handler().Handle(ctx, rec)

	return nil
}

// Enabled checks if the given log level is enabled.
func (h *slogHandler) Enabled(level handler.LogLevel) bool {
	return h.base.Enabled(level)
}

// HandlerState returns the underlying BaseHandler.
func (h *slogHandler) HandlerState() handler.HandlerState {
	return h.base
}

// Features returns the supported HandlerFeatures.
func (h *slogHandler) Features() handler.HandlerFeatures {
	return handler.NewHandlerFeatures(
		handler.FeatNativeCaller | // slog.HandlerOptions.AddSource
			handler.FeatNativeGroup | // slog.Handler.WithGroup
			handler.FeatContextPropagation | // slog.Handler.Handle(ctx)
			handler.FeatDynamicLevel |
			handler.FeatDynamicOutput)
}

// WithAttrs returns a new logger with the provided keyValues added to the context.
// If keyValues is empty, the original logger is returned.
func (h *slogHandler) WithAttrs(keyValues []any) handler.Chainer {
	attrs := keyValuesToSlogAttrs(keyValues)
	if len(attrs) == 0 {
		return h
	}

	clone := h.clone()
	clone.handler = h.handler.WithAttrs(attrs)
	clone.logger = slog.New(clone.handler)

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (h *slogHandler) WithGroup(name string) handler.Chainer {
	if name == "" {
		return h
	}

	clone := h.clone()
	clone.handler = h.handler.WithGroup(name)
	clone.logger = slog.New(clone.handler)

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (h *slogHandler) SetLevel(level handler.LogLevel) error {
	if err := h.base.SetLevel(level); err != nil {
		return err
	}

	h.level.Set(levelMapper.Map(level))

	return nil
}

// SetOutput sets the log destination.
func (h *slogHandler) SetOutput(w io.Writer) error {
	return h.base.SetOutput(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (h *slogHandler) CallerSkip() int {
	return h.base.CallerSkip()
}

// WithCaller returns a new handler with caller reporting enabled or disabled.
// It returns the original handler if the enabled value is unchanged.
func (h *slogHandler) WithCaller(enabled bool) handler.AdvancedHandler {
	newBase := h.base.WithCaller(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithTrace returns a new handler that enables or disables stack trace logging.
// It returns the original handler if the enabled value is unchanged.
func (h *slogHandler) WithTrace(enabled bool) handler.AdvancedHandler {
	newBase := h.base.WithTrace(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithLevel returns a new handler with a new minimum level applied.
// It returns the original handler if the level value is unchanged.
func (h *slogHandler) WithLevel(level handler.LogLevel) handler.AdvancedHandler {
	newBase, err := h.base.WithLevel(level)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithOutput returns a new handler with the output writer set permanently.
// It returns the original handler if the writer value is unchanged.
func (h *slogHandler) WithOutput(w io.Writer) handler.AdvancedHandler {
	newBase, err := h.base.WithOutput(w)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithCallerSkip returns a new handler with the caller skip permanently adjusted.
// It returns the original handler if the skip value is unchanged.
func (h *slogHandler) WithCallerSkip(skip int) handler.AdvancedHandler {
	current := h.base.CallerSkip()
	if skip == current {
		return h
	}

	return h.WithCallerSkipDelta(skip - current)
}

// WithCallerSkipDelta returns a new handler with the caller skip altered by delta.
// It returns the original handler if the delta value is zero.
func (h *slogHandler) WithCallerSkipDelta(delta int) handler.AdvancedHandler {
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
func (h *slogHandler) clone() *slogHandler {
	return &slogHandler{
		base:        h.base,
		logger:      h.logger,
		level:       h.level,
		handler:     h.handler,
		replaceAttr: h.replaceAttr,
		withCaller:  h.withCaller,
		withTrace:   h.withTrace,
	}
}

// deepClone returns a deep copy of the logger with a new BaseHandler.
func (h *slogHandler) deepClone(base *handler.BaseHandler) *slogHandler {
	levelVar := new(slog.LevelVar)
	levelVar.Set(levelMapper.Map(base.Level()))

	handlerOpts := &slog.HandlerOptions{
		Level:       levelVar,
		AddSource:   base.CallerEnabled(),
		ReplaceAttr: h.replaceAttr,
	}

	var sh slog.Handler
	if base.Format() == "text" {
		sh = slog.NewTextHandler(base.AtomicWriter(), handlerOpts)
	} else {
		sh = slog.NewJSONHandler(base.AtomicWriter(), handlerOpts)
	}

	return &slogHandler{
		base:        base,
		logger:      slog.New(sh),
		level:       levelVar,
		handler:     sh,
		replaceAttr: h.replaceAttr,
		withCaller:  base.CallerEnabled(),
		withTrace:   base.TraceEnabled(),
	}
}

// keyValuesToSlogAttrs transforms keyValues to slog.Attrs.
func keyValuesToSlogAttrs(keyValues []any) []slog.Attr {
	n := len(keyValues)
	attrCount := n / 2

	if attrCount == 0 {
		return nil
	}

	// Stack-allocate for common case (≤6 attributes)
	const stackN = 6
	var stackAttrs [stackN]slog.Attr
	var attrs []slog.Attr
	if attrCount <= stackN {
		attrs = stackAttrs[:0]
	} else {
		attrs = make([]slog.Attr, 0, attrCount)
	}

	for i := 0; i < n-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprint(keyValues[i])
		}
		attrs = append(attrs, slog.Any(key, keyValues[i+1]))
	}

	return attrs
}
