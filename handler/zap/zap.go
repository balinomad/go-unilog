package zap

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/balinomad/go-unilog/handler"
)

// validFormats is the list of supported output formats.
var validFormats = []string{"json", "console"}

// zapOptions holds configuration for the Zap logger.
type zapOptions struct {
	base *handler.BaseOptions
}

// ZapOption configures the Zap logger creation.
type ZapOption func(*zapOptions) error

// WithLevel sets the minimum log level.
func WithLevel(level handler.LogLevel) ZapOption {
	return func(o *zapOptions) error {
		return handler.WithLevel(level)(o.base)
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) ZapOption {
	return func(o *zapOptions) error {
		return handler.WithOutput(w)(o.base)
	}
}

// WithCaller enables or disables source location reporting.
// If enabled, the handler will include the source location
// of the log call site in the log record.
// This can be useful for debugging, but may incur a performance hit
// due to the additional stack frame analysis. The default value is false.
func WithCaller(enabled bool) ZapOption {
	return func(o *zapOptions) error {
		return handler.WithCaller(enabled)(o.base)
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) ZapOption {
	return func(o *zapOptions) error {
		return handler.WithTrace(enabled)(o.base)
	}
}

// zapHandler is a wrapper around Zap's logger.
type zapHandler struct {
	base           *handler.BaseHandler
	logger         *zap.Logger
	atomicLevel    zap.AtomicLevel
	encoderFactory func() zapcore.Encoder
	writeSyncer    zapcore.WriteSyncer
	zapOpts        []zap.Option

	// Cached from BaseHandler for lock-free hot-path
	withCaller bool
	withTrace  bool
	callerSkip int
}

// Ensure zapHandler implements all interfaces explicitly.
var (
	_ handler.Handler         = (*zapHandler)(nil)
	_ handler.Chainer         = (*zapHandler)(nil)
	_ handler.AdvancedHandler = (*zapHandler)(nil)
	_ handler.Configurator    = (*zapHandler)(nil)
	_ handler.Syncer          = (*zapHandler)(nil)
)

// levelMapper maps unilog log levels to zap log levels.
var levelMapper = handler.NewLevelMapper(
	zapcore.DebugLevel, // Trace
	zapcore.DebugLevel, // Debug
	zapcore.InfoLevel,  // Info
	zapcore.WarnLevel,  // Warn
	zapcore.ErrorLevel, // Error
	zapcore.ErrorLevel, // Critical
	zapcore.FatalLevel, // Fatal
	zapcore.PanicLevel, // Panic
)

// New creates a new handler.Handler instance backed by zap.
// It also captures enough internal pieces to be able to recreate/clone
// the embedded zap.Logger later with a different set of options.
func New(opts ...ZapOption) (handler.Handler, error) {
	o := &zapOptions{
		base: &handler.BaseOptions{
			Level:        handler.DefaultLevel,
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

	// Create the write syncer once and keep it for future clones
	writeSyncer := zapcore.AddSync(base.AtomicWriter())

	// Create the initial atomic level and keep a value copy
	initialLevel := zap.NewAtomicLevelAt(levelMapper.Map(base.Level()))

	// Build encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create an encoderFactory so we can reproduce the same encoder later
	var encoderFactory func() zapcore.Encoder
	if base.Format() == "console" {
		encoderFactory = func() zapcore.Encoder {
			return zapcore.NewConsoleEncoder(encoderConfig)
		}
	} else {
		encoderFactory = func() zapcore.Encoder {
			return zapcore.NewJSONEncoder(encoderConfig)
		}
	}

	// Build initial core and zap options
	core := zapcore.NewCore(encoderFactory(), writeSyncer, initialLevel)
	zapOpts := buildZapOpts(base)
	zl := zap.New(core, zapOpts...)

	return &zapHandler{
		base:           base,
		logger:         zl,
		atomicLevel:    initialLevel,
		encoderFactory: encoderFactory,
		writeSyncer:    writeSyncer,
		zapOpts:        zapOpts,
		withCaller:     base.CallerEnabled(),
		withTrace:      base.TraceEnabled(),
		callerSkip:     base.CallerSkip(),
	}, nil
}

// Handle implements the handler.Handler interface for zap.
func (h *zapHandler) Handle(ctx context.Context, r *handler.Record) error {
	if !h.Enabled(r.Level) {
		return nil
	}

	zl := h.logger

	// Apply per-record dynamic skip
	if h.withCaller && r.Skip != 0 {
		zl = zl.WithOptions(zap.AddCallerSkip(r.Skip))
	}

	ce := zl.Check(levelMapper.Map(r.Level), r.Message)
	if ce == nil {
		return nil
	}

	ce.Write(keyValuesToZapFields(r.KeyValues)...)

	return nil
}

// Enabled checks if the given log level is enabled.
func (h *zapHandler) Enabled(level handler.LogLevel) bool {
	return h.base.Enabled(level)
}

// Base returns the underlying BaseHandler.
func (h *zapHandler) HandlerState() handler.HandlerState {
	return h.base
}

// Features returns the supported HandlerFeatures.
func (h *zapHandler) Features() handler.HandlerFeatures {
	return handler.NewHandlerFeatures(
		handler.FeatNativeCaller | // zap.AddCallerSkip
			handler.FeatNativeGroup | // zap.Namespace
			handler.FeatBufferedOutput | // zap.Sync()
			handler.FeatDynamicLevel | // zap.AtomicLevel
			handler.FeatDynamicOutput) // handler.BaseHandler.AtomicWriter
}

// WithAttrs returns a child handler with the provided keyValues added to the context.
// If keyValues is empty, the original handler is returned.
func (h *zapHandler) WithAttrs(keyValues []any) handler.Chainer {
	fields := keyValuesToZapFields(keyValues)
	if len(fields) == 0 {
		return h
	}

	clone := h.clone()
	clone.logger = h.logger.With(fields...)

	return clone
}

// WithGroup returns a handler that starts a group, if name is non-empty.
func (h *zapHandler) WithGroup(name string) handler.Chainer {
	if name == "" {
		return h
	}

	clone := h.clone()
	clone.logger = h.logger.With(zap.Namespace(name))

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (h *zapHandler) SetLevel(level handler.LogLevel) error {
	// Validate and store in base
	if err := h.base.SetLevel(level); err != nil {
		return err
	}

	h.atomicLevel.SetLevel(levelMapper.Map(level))

	return nil
}

// SetOutput changes the destination for log output.
func (h *zapHandler) SetOutput(w io.Writer) error {
	return h.base.SetOutput(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (h *zapHandler) CallerSkip() int {
	return h.base.CallerSkip()
}

// WithCaller returns a new handler with caller reporting enabled or disabled.
// It returns the original handler if the enabled value is unchanged.
func (h *zapHandler) WithCaller(enabled bool) handler.AdvancedHandler {
	newBase := h.base.WithCaller(enabled)
	if newBase == h.base {
		return h
	}

	// Rebuild zapOpts with new caller state
	newZapOpts := buildZapOpts(newBase)

	clone := h.clone()
	clone.base = newBase
	clone.logger = zap.New(zapcore.NewCore(h.encoderFactory(), h.writeSyncer, h.atomicLevel), newZapOpts...)
	clone.zapOpts = newZapOpts
	clone.withCaller = enabled

	return clone
}

// WithTrace returns a new handler that enables or disables stack trace logging for error-level logs.
// It returns the original handler if the enabled value is unchanged.
func (h *zapHandler) WithTrace(enabled bool) handler.AdvancedHandler {
	newBase := h.base.WithTrace(enabled)
	if newBase == h.base {
		return h
	}

	// Rebuild zapOpts with new trace state
	newZapOpts := buildZapOpts(newBase)

	clone := h.clone()
	clone.base = newBase
	clone.logger = zap.New(zapcore.NewCore(h.encoderFactory(), h.writeSyncer, h.atomicLevel), newZapOpts...)
	clone.zapOpts = newZapOpts
	clone.withTrace = enabled

	return clone
}

// WithLevel returns a new Zap handler with a new minimum level applied.
// It returns the original handler if the level value is unchanged.
func (h *zapHandler) WithLevel(level handler.LogLevel) handler.AdvancedHandler {
	newBase, err := h.base.WithLevel(level)
	if err != nil || newBase == h.base {
		return h
	}

	newLevel := zap.NewAtomicLevelAt(levelMapper.Map(level))
	newZapOpts := make([]zap.Option, len(h.zapOpts))
	copy(newZapOpts, h.zapOpts)

	return &zapHandler{
		base: newBase,
		logger: zap.New(
			zapcore.NewCore(h.encoderFactory(), h.writeSyncer, newLevel),
			newZapOpts...),
		atomicLevel:    newLevel,
		encoderFactory: h.encoderFactory,
		writeSyncer:    h.writeSyncer,
		zapOpts:        newZapOpts,
		withCaller:     h.withCaller,
		withTrace:      h.withTrace,
		callerSkip:     h.callerSkip,
	}
}

// WithOutput returns a new handler with the output writer set permanently.
// It returns the original handler if the writer value is unchanged.
func (h *zapHandler) WithOutput(w io.Writer) handler.AdvancedHandler {
	_ = h.logger.Sync()

	newBase, err := h.base.WithOutput(w)
	if err != nil || newBase == h.base {
		return h
	}

	newWriteSyncer := zapcore.AddSync(newBase.AtomicWriter())
	newAtomicLevel := zap.NewAtomicLevelAt(h.atomicLevel.Level())
	newZapOpts := make([]zap.Option, len(h.zapOpts))
	copy(newZapOpts, h.zapOpts)

	return &zapHandler{
		base: newBase,
		logger: zap.New(
			zapcore.NewCore(h.encoderFactory(), newWriteSyncer, newAtomicLevel),
			newZapOpts...),
		atomicLevel:    newAtomicLevel,
		encoderFactory: h.encoderFactory,
		writeSyncer:    newWriteSyncer,
		zapOpts:        newZapOpts,
		withCaller:     h.withCaller,
		withTrace:      h.withTrace,
		callerSkip:     h.callerSkip,
	}
}

// WithCallerSkip returns a new handler with the caller skip permanently adjusted.
func (h *zapHandler) WithCallerSkip(skip int) handler.AdvancedHandler {
	current := h.base.CallerSkip()
	if skip == current {
		return h
	}

	return h.WithCallerSkipDelta(skip - current)
}

// WithCallerSkipDelta returns a new handler with the caller skip permanently adjusted by delta.
func (h *zapHandler) WithCallerSkipDelta(delta int) handler.AdvancedHandler {
	if delta == 0 {
		return h
	}

	baseClone, err := h.base.WithCallerSkipDelta(delta)
	if err != nil {
		return h
	}

	return &zapHandler{
		base:           baseClone,
		logger:         h.logger.WithOptions(zap.AddCallerSkip(delta)),
		atomicLevel:    h.atomicLevel,
		encoderFactory: h.encoderFactory,
		writeSyncer:    h.writeSyncer,
		zapOpts:        h.zapOpts,
		withCaller:     h.withCaller,
		withTrace:      h.withTrace,
		callerSkip:     baseClone.CallerSkip(),
	}
}

// Sync flushes any buffered log entries.
func (h *zapHandler) Sync() error {
	return h.logger.Sync()
}

// clone returns a shallow copy for immutable chaining.
func (h *zapHandler) clone() *zapHandler {
	return &zapHandler{
		base:           h.base,
		logger:         h.logger,
		atomicLevel:    h.atomicLevel,
		encoderFactory: h.encoderFactory,
		writeSyncer:    h.writeSyncer,
		zapOpts:        h.zapOpts,
		withCaller:     h.withCaller,
		withTrace:      h.withTrace,
		callerSkip:     h.callerSkip,
	}
}

// buildZapOpts creates zap.Option slice from BaseHandler state.
func buildZapOpts(base *handler.BaseHandler) []zap.Option {
	opts := make([]zap.Option, 0, 2)
	if base.CallerEnabled() {
		// AddCallerSkip needs to account for our adapter's internal frames
		opts = append(opts, zap.AddCaller(), zap.AddCallerSkip(base.CallerSkip()))
	}
	if base.TraceEnabled() {
		// Adds stack trace to logs at Error level and above
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}
	return opts
}

// keyValuesToZapFields transforms keyValues to zap.Fields.
func keyValuesToZapFields(keyValues []any) []zap.Field {
	n := len(keyValues)
	fieldCount := n / 2

	if fieldCount == 0 {
		return nil
	}

	// Stack-allocate for common case (≤6 attributes)
	const stackN = 6
	var stackFields [stackN]zap.Field
	var fields []zap.Field
	if fieldCount <= stackN {
		fields = stackFields[:0]
	} else {
		fields = make([]zap.Field, 0, fieldCount)
	}

	for i := 0; i < n-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprint(keyValues[i])
		}
		fields = append(fields, attrToZapField(key, keyValues[i+1]))
	}

	return fields
}

// attrToZapField handles the most frequently logged concrete types and falls
// back to zap.Any for the rest.
func attrToZapField(key string, v any) zap.Field {
	if v == nil {
		return zap.Any(key, nil)
	}

	switch vv := v.(type) {
	case string:
		return zap.String(key, vv)
	case int:
		return zap.Int(key, vv)
	case error:
		return zap.NamedError(key, vv)
	case bool:
		return zap.Bool(key, vv)
	case int64:
		return zap.Int64(key, vv)
	case float64:
		return zap.Float64(key, vv)
	case time.Time:
		return zap.Time(key, vv)
	case time.Duration:
		return zap.Duration(key, vv)
	case uint64:
		return zap.Uint64(key, vv)
	case uint:
		return zap.Uint(key, vv)
	case int8:
		return zap.Int8(key, vv)
	case int16:
		return zap.Int16(key, vv)
	case []byte:
		return zap.ByteString(key, vv)
	default:
		return zap.Any(key, vv)
	}
}
