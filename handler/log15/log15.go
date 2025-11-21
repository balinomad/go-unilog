package log15

import (
	"context"
	"io"
	"os"
	"runtime/debug"

	"github.com/inconshreveable/log15/v3"

	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-unilog/handler"
)

// validFormats is the list of supported output formats.
var validFormats = []string{"json", "terminal", "logfmt"}

// defaultFormat is the default output format.
var defaultFormat = "terminal"

// log15Options holds configuration for the log15 logger.
type log15Options struct {
	base *handler.BaseOptions
}

// Log15Option configures log15 logger creation.
type Log15Option func(*log15Options) error

// WithLevel sets the minimum log level.
func WithLevel(level handler.LogLevel) Log15Option {
	return func(o *log15Options) error {
		return handler.WithLevel(level)(o.base)
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) Log15Option {
	return func(o *log15Options) error {
		return handler.WithOutput(w)(o.base)
	}
}

// WithFormat sets the output format ("json", "terminal", or "logfmt").
// The default format is "terminal".
func WithFormat(format string) Log15Option {
	return func(o *log15Options) error {
		return handler.WithFormat(format)(o.base)
	}
}

// WithCaller enables or disables source location reporting.
// If enabled, the handler will include the source location
// of the log call site in the log record.
// This can be useful for debugging, but may incur a performance hit
// due to the additional stack frame analysis. The default value is false.
func WithCaller(enabled bool) Log15Option {
	return func(o *log15Options) error {
		return handler.WithCaller(enabled)(o.base)
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) Log15Option {
	return func(o *log15Options) error {
		return handler.WithTrace(enabled)(o.base)
	}
}

// log15Handler is a wrapper around log15 package.
type log15Handler struct {
	base      *handler.BaseHandler
	logger    log15.Logger
	keyValues []any

	// Cached from base for lock-free hot-path
	withCaller bool
	withTrace  bool
}

// Ensure log15Handler implements the following interfaces.
var (
	_ handler.Handler        = (*log15Handler)(nil)
	_ handler.Chainer        = (*log15Handler)(nil)
	_ handler.Configurable   = (*log15Handler)(nil)
	_ handler.CallerAdjuster = (*log15Handler)(nil)
	_ handler.FeatureToggler = (*log15Handler)(nil)
	_ handler.MutableConfig  = (*log15Handler)(nil)
)

// levelMapper maps unilog log levels to log15 log levels.
var levelMapper = handler.NewLevelMapper(
	log15.LvlDebug, // Trace (no native equivalent)
	log15.LvlDebug, // Debug
	log15.LvlInfo,  // Info
	log15.LvlWarn,  // Warn
	log15.LvlError, // Error
	log15.LvlCrit,  // Critical
	log15.LvlCrit,  // Fatal (map to Crit, exit handled by unilog)
	log15.LvlCrit,  // Panic (map to Crit, panic handled by unilog)
)

// New creates a new handler.Handler instance backed by log15.
func New(opts ...Log15Option) (handler.Handler, error) {
	o := &log15Options{
		base: &handler.BaseOptions{
			Level:        handler.DefaultLevel,
			Output:       os.Stderr,
			Format:       defaultFormat,
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

	h := &log15Handler{
		base:       base,
		logger:     log15.New(),
		keyValues:  []any{},
		withCaller: base.CallerEnabled(),
		withTrace:  base.TraceEnabled(),
	}
	h.setLog15Logger()

	return h, nil
}

// Handle implements the handler.Handler interface for log15.
func (h *log15Handler) Handle(ctx context.Context, r *handler.Record) error {
	if !h.Enabled(r.Level) {
		return nil
	}

	// Combine handler attributes + record attributes
	fields := make([]any, 0, len(h.keyValues)+len(r.KeyValues)+4)
	fields = append(fields, h.keyValues...)
	fields = append(fields, r.KeyValues...)

	// Only compute caller if enabled
	if h.withCaller && r.PC != 0 {
		fields = append(fields, "source", caller.NewFromPC(r.PC).Location())
	}

	// Only capture stack if enabled and error-level
	if h.withTrace && r.Level >= handler.ErrorLevel {
		fields = append(fields, "stack", string(debug.Stack()))
	}

	h.logger.GetHandler().Log(
		log15.Record{
			Time:     r.Time,
			Lvl:      levelMapper.Map(r.Level),
			Msg:      r.Message,
			Ctx:      fields,
			KeyNames: log15.DefaultRecordKeyNames,
		})

	return nil
}

// Enabled checks if the given log level is enabled.
func (h *log15Handler) Enabled(level handler.LogLevel) bool {
	return h.base.Enabled(level)
}

// HandlerState returns the underlying BaseHandler.
func (h *log15Handler) HandlerState() handler.HandlerState {
	return h.base
}

// Features returns the supported HandlerFeatures.
func (h *log15Handler) Features() handler.HandlerFeatures {
	return handler.NewHandlerFeatures(handler.FeatDynamicLevel | handler.FeatDynamicOutput)
}

// WithAttrs returns a new logger with the provided keyValues added to the context.
// If keyValues is empty, the original logger is returned.
func (h *log15Handler) WithAttrs(keyValues []any) handler.Chainer {
	if len(keyValues) < 2 {
		return h
	}

	clone := h.clone()

	// Fast merge of keyValues
	lenOld := len(clone.keyValues)
	lenNew := len(keyValues)
	if lenNew%2 != 0 {
		lenNew--
	}
	merged := make([]any, lenOld+lenNew)
	copy(merged, clone.keyValues)
	copy(merged[lenOld:], keyValues[:lenNew])
	clone.keyValues = merged

	return clone
}

// WithGroup returns a Logger that starts a group (via key prefixing).
func (h *log15Handler) WithGroup(name string) handler.Chainer {
	if name == "" {
		return h
	}

	base, err := h.base.WithKeyPrefix(name)
	if err == nil {
		return h
	}

	clone := h.clone()
	clone.base = base

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (h *log15Handler) SetLevel(level handler.LogLevel) error {
	if err := h.base.SetLevel(level); err != nil {
		return err
	}

	h.logger.SetHandler(h.buildLog15Handler())

	return nil
}

// SetOutput sets the log destination.
func (h *log15Handler) SetOutput(w io.Writer) error {
	if err := h.base.SetOutput(w); err != nil {
		return err
	}

	h.logger.SetHandler(h.buildLog15Handler())

	return nil
}

// CallerSkip returns the current number of stack frames being skipped.
func (h *log15Handler) CallerSkip() int {
	return h.base.CallerSkip()
}

// WithCaller returns a new handler with caller reporting enabled or disabled.
func (h *log15Handler) WithCaller(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithCaller(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithTrace returns a new handler that enables or disables stack trace logging.
func (h *log15Handler) WithTrace(enabled bool) handler.FeatureToggler {
	newBase := h.base.WithTrace(enabled)
	if newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithLevel returns a new handler with a new minimum level applied.
func (h *log15Handler) WithLevel(level handler.LogLevel) handler.Configurable {
	newBase, err := h.base.WithLevel(level)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithOutput returns a new handler with the output writer set permanently.
func (h *log15Handler) WithOutput(w io.Writer) handler.Configurable {
	newBase, err := h.base.WithOutput(w)
	if err != nil || newBase == h.base {
		return h
	}

	return h.deepClone(newBase)
}

// WithCallerSkip returns a new handler with the caller skip permanently adjusted.
func (h *log15Handler) WithCallerSkip(skip int) handler.CallerAdjuster {
	current := h.base.CallerSkip()
	if skip == current {
		return h
	}

	return h.WithCallerSkipDelta(skip - current)
}

// WithCallerSkipDelta returns a new handler with the caller skip altered by delta.
func (h *log15Handler) WithCallerSkipDelta(delta int) handler.CallerAdjuster {
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
func (h *log15Handler) clone() *log15Handler {
	return &log15Handler{
		base:       h.base,
		logger:     h.logger,
		keyValues:  h.keyValues,
		withCaller: h.withCaller,
		withTrace:  h.withTrace,
	}
}

// deepClone returns a deep copy of the logger with a new BaseHandler.
func (h *log15Handler) deepClone(base *handler.BaseHandler) *log15Handler {
	kvClone := make([]any, len(h.keyValues))
	copy(kvClone, h.keyValues)

	clone := &log15Handler{
		base:       base,
		keyValues:  kvClone,
		withCaller: base.CallerEnabled(),
		withTrace:  base.TraceEnabled(),
	}
	clone.setLog15Logger()

	return clone
}

// buildLog15Handler returns a log15.Handler based on the current configuration.
func (h *log15Handler) buildLog15Handler() log15.Handler {
	format := stringToFormat(h.base.Format())
	handler := log15.StreamHandler(h.base.AtomicWriter(), format)
	return log15.LvlFilterHandler(levelMapper.Map(h.base.Level()), handler)
}

// setLog15Logger sets the log15.Logger to be used by the handler.
// Handler must have a valid BaseHandler.
func (h *log15Handler) setLog15Logger() {
	h.logger = log15.New()
	h.logger.SetHandler(h.buildLog15Handler())
}

// stringToFormat returns a log15.Format based on the given format string.
func stringToFormat(format string) log15.Format {
	switch format {
	case "json":
		return log15.JsonFormat()
	case "logfmt":
		return log15.LogfmtFormat()
	case "terminal":
		return log15.TerminalFormat()
	default:
		return log15.TerminalFormat()
	}
}
