package stdlog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-unilog/handler"
)

// stdLogOptions holds configuration for the standard logger.
type stdLogOptions struct {
	base  *handler.BaseOptions
	flags int // log.Ldate | log.Ltime | log.Lmicroseconds, etc.
}

// StdLogOption configures the standard logger creation.
type StdLogOption func(*stdLogOptions) error

// WithLevel sets the minimum log level.
func WithLevel(level handler.LogLevel) StdLogOption {
	return func(o *stdLogOptions) error {
		return handler.WithLevel(level)(o.base)
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) StdLogOption {
	return func(o *stdLogOptions) error {
		return handler.WithOutput(w)(o.base)
	}
}

// WithSeparator sets the separator for group key prefixes.
func WithSeparator(separator string) StdLogOption {
	return func(o *stdLogOptions) error {
		return handler.WithSeparator(separator)(o.base)
	}
}

// WithCaller enables or disables source location reporting.
// If enabled, the handler will include the source location
// of the log call site in the log record.
// This can be useful for debugging, but may incur a performance hit
// due to the additional stack frame analysis. The default value is false.
func WithCaller(enabled bool) StdLogOption {
	return func(o *stdLogOptions) error {
		return handler.WithCaller(enabled)(o.base)
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) StdLogOption {
	return func(o *stdLogOptions) error {
		return handler.WithTrace(enabled)(o.base)
	}
}

// WithFlags sets the log flags.
func WithFlags(flags int) StdLogOption {
	return func(o *stdLogOptions) error {
		o.flags = flags
		return nil
	}
}

// stdLogHandler is a wrapper around Go's standard library log package.
type stdLogHandler struct {
	base      *handler.BaseHandler
	logger    *log.Logger
	keyValues []any // Pre-formatted keys: "prefix_key", value...

	// Cached from base for lock-free hot-path
	withCaller bool
	withTrace  bool
	separator  string
}

// Ensure stdLogHandler implements the following interfaces.
var (
	_ handler.Handler        = (*stdLogHandler)(nil)
	_ handler.Chainer        = (*stdLogHandler)(nil)
	_ handler.Configurable   = (*stdLogHandler)(nil)
	_ handler.CallerAdjuster = (*stdLogHandler)(nil)
	_ handler.FeatureToggler = (*stdLogHandler)(nil)
	_ handler.MutableConfig  = (*stdLogHandler)(nil)
)

// New creates a new handler.Handler instance backed by the standard log.
func New(opts ...StdLogOption) (handler.Handler, error) {
	o := &stdLogOptions{
		base: &handler.BaseOptions{
			Level:  handler.DefaultLevel,
			Output: os.Stderr,
		},
		flags: log.LstdFlags,
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

	return &stdLogHandler{
		base:       base,
		logger:     log.New(base.AtomicWriter(), "", o.flags),
		keyValues:  nil,
		withCaller: base.CallerEnabled(),
		withTrace:  base.TraceEnabled(),
	}, nil
}

// Handle implements the handler.Handler interface for the standard logger.
func (h *stdLogHandler) Handle(_ context.Context, r *handler.Record) error {
	if !h.Enabled(r.Level) {
		return nil
	}

	// Heuristic pre-allocation: message + existing attrs + new attrs + overhead
	estSize := len(r.Message) + len(h.keyValues)*10 + len(r.KeyValues)*10 + 50
	var sb strings.Builder
	sb.Grow(estSize)

	// Level prefix
	sb.WriteString("[")
	sb.WriteString(r.Level.String())
	sb.WriteString("] ")
	sb.WriteString(r.Message)

	// Write baked-in attributes (prefixes already applied)
	writePairs(&sb, h.keyValues)

	// Write record attributes (apply current prefix)
	currentPrefix := h.base.KeyPrefix()
	separator := h.base.Separator()

	for i := 0; i < len(r.KeyValues)-1; i += 2 {
		sb.WriteString(" ")
		if currentPrefix != "" {
			sb.WriteString(currentPrefix)
			sb.WriteString(separator)
		}
		key, ok := r.KeyValues[i].(string)
		if !ok {
			key = fmt.Sprint(r.KeyValues[i])
		}
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprint(r.KeyValues[i+1]))
	}

	// Only compute caller if enabled
	if h.withCaller && r.PC != 0 {
		sb.WriteString(" source=")
		sb.WriteString(caller.NewFromPC(r.PC).Location())
	}

	// Only capture stack if enabled and error-level
	if h.withTrace && r.Level >= handler.ErrorLevel {
		sb.WriteString(" stack=")
		sb.WriteString(string(debug.Stack()))
	}

	h.logger.Println(sb.String())

	return nil
}

// Enabled checks if the given log level is enabled.
func (h *stdLogHandler) Enabled(level handler.LogLevel) bool {
	return h.base.Enabled(level)
}

// HandlerState returns the underlying BaseHandler.
func (h *stdLogHandler) HandlerState() handler.HandlerState {
	return h.base
}

// Features returns the supported HandlerFeatures.
func (h *stdLogHandler) Features() handler.HandlerFeatures {
	return handler.NewHandlerFeatures(handler.FeatDynamicLevel | handler.FeatDynamicOutput)
}

// WithAttrs returns a new logger with the provided keyValues added to the context.
// If keyValues is empty, the original logger is returned.
func (h *stdLogHandler) WithAttrs(keyValues []any) handler.Chainer {
	if len(keyValues) < 2 {
		return h
	}

	clone := h.clone()

	// Bake prefix into new keys immediately
	prefix := h.base.KeyPrefix()
	sep := h.base.Separator()

	// New slice size = old + new
	newAttrs := make([]any, len(h.keyValues)+len(keyValues))
	copy(newAttrs, h.keyValues)

	// Append new items, formatting keys if needed
	dest := newAttrs[len(h.keyValues):]
	for i := 0; i < len(keyValues)-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprint(keyValues[i])
		}
		if prefix != "" {
			key = prefix + sep + key
		}
		dest[i] = key
		dest[i+1] = keyValues[i+1]
	}

	clone.keyValues = newAttrs

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (h *stdLogHandler) WithGroup(name string) handler.Chainer {
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
func (h *stdLogHandler) SetLevel(level handler.LogLevel) error {
	return h.base.SetLevel(level)
}

// SetOutput sets the log destination.
func (h *stdLogHandler) SetOutput(w io.Writer) error {
	return h.base.SetOutput(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (h *stdLogHandler) CallerSkip() int {
	return h.base.CallerSkip()
}

// WithCaller returns a new handler with caller reporting enabled or disabled.
// It returns the original handler if the enabled value is unchanged.
func (h *stdLogHandler) WithCaller(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithCaller(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithTrace returns a new handler that enables or disables stack trace logging.
// It returns the original handler if the enabled value is unchanged.
func (h *stdLogHandler) WithTrace(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithTrace(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithLevel returns a new handler with a new minimum level applied.
// It returns the original handler if the level value is unchanged.
func (h *stdLogHandler) WithLevel(level handler.LogLevel) handler.Configurable {
	newBase, err := h.base.WithLevel(level)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithOutput returns a new handler with the output writer set permanently.
// It returns the original handler if the writer value is unchanged.
func (h *stdLogHandler) WithOutput(w io.Writer) handler.Configurable {
	newBase, err := h.base.WithOutput(w)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithCallerSkip returns a new handler with the caller skip permanently adjusted.
// It returns the original handler if the skip value is unchanged.
func (h *stdLogHandler) WithCallerSkip(skip int) handler.CallerAdjuster {
	current := h.base.CallerSkip()
	if skip == current {
		return h
	}

	return h.WithCallerSkipDelta(skip - current)
}

// WithCallerSkipDelta returns a new handler with the caller skip altered by delta.
// It returns the original handler if the delta value is zero.
func (h *stdLogHandler) WithCallerSkipDelta(delta int) handler.CallerAdjuster {
	if delta == 0 {
		return h
	}

	newBase, err := h.base.WithCallerSkipDelta(delta)
	if err != nil {
		return h
	}

	return h.deepClone(newBase)
}

// clone returns a shallow copy of the logger.
func (h *stdLogHandler) clone() *stdLogHandler {
	return &stdLogHandler{
		base:       h.base,
		logger:     h.logger,
		keyValues:  h.keyValues,
		withCaller: h.withCaller,
		withTrace:  h.withTrace,
		separator:  h.separator,
	}
}

// deepClone returns a deep copy of the logger with a new BaseHandler.
func (h *stdLogHandler) deepClone(base *handler.BaseHandler) *stdLogHandler {
	kv := make([]any, len(h.keyValues))
	copy(kv, h.keyValues)

	return &stdLogHandler{
		base:       base,
		logger:     log.New(base.AtomicWriter(), "", h.logger.Flags()),
		keyValues:  kv,
		withCaller: base.CallerEnabled(),
		withTrace:  base.TraceEnabled(),
		separator:  base.Separator(),
	}
}

// writePairs writes key-value pairs to the provided strings.Builder.
func writePairs(sb *strings.Builder, keyValues []any) {
	for i := 0; i < len(keyValues)-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprint(keyValues[i])
		}
		sb.WriteString(" ")
		sb.WriteString(key)
		sb.WriteString("=")
		fmt.Fprint(sb, keyValues[i+1])
	}
}
