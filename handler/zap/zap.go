package zap

import (
	"context"
	"io"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/balinomad/go-unilog/handler"
)

// internalSkipFrames is the number of stack frames this handler adds
// between zapHandler.Handle() and the backend logger call.
//
// Frames to skip:
//
//	1:  zapHandler.Handle()
const internalSkipFrames = 1

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

// WithCaller enables source location reporting.
// The optional skip parameter adjusts the reported call site by skipping
// additional stack frames beyond the handler's internal frames.
//
// Example:
//
//	handler := New(WithCaller(true))        // Reports actual call site
//	handler := New(WithCaller(true, 1))     // Skip 1 frame (for wrapper)
func WithCaller(enabled bool, skip ...int) ZapOption {
	return func(o *zapOptions) error {
		return handler.WithCaller(enabled, skip...)(o.base)
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
	base        *handler.BaseHandler
	logger      *zap.Logger
	atomicLevel zap.AtomicLevel
}

// Ensure zapHandler implements the following interfaces.
var (
	_ handler.Handler      = (*zapHandler)(nil)
	_ handler.Chainer      = (*zapHandler)(nil)
	_ handler.Configurator = (*zapHandler)(nil)
	_ handler.Syncer       = (*zapHandler)(nil)
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
	o.base.CallerSkip += internalSkipFrames

	base, err := handler.NewBaseHandler(o.base)
	if err != nil {
		return nil, err
	}

	writeSyncer := zapcore.AddSync(base.AtomicWriter())
	atomicLevel := zap.NewAtomicLevelAt(levelMapper.Map(base.Level()))

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	var encoder zapcore.Encoder
	if base.Format() == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, writeSyncer, atomicLevel)

	// Build zap options natively
	zapOpts := make([]zap.Option, 0, 2)
	if base.CallerEnabled() {
		// AddCallerSkip needs to account for our adapter's internal frames
		zapOpts = append(zapOpts, zap.AddCaller(), zap.AddCallerSkip(base.CallerSkip()))
	}
	if base.TraceEnabled() {
		// Adds stack trace to logs at Error level and above
		zapOpts = append(zapOpts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	zl := zap.New(core, zapOpts...)

	return &zapHandler{
		base:        base,
		logger:      zl,
		atomicLevel: atomicLevel,
	}, nil
}

// LHandleog implements the handler.Handler interface for zap.
func (h *zapHandler) Handle(ctx context.Context, r *handler.Record) error {
	if !h.Enabled(r.Level) {
		return nil
	}

	base := h.base
	zl := h.logger

	// Apply per-record dynamic skip
	if base.CallerEnabled() {
		skip := max(r.Skip, 0)
		if skip > 0 {
			zl = zl.WithOptions(zap.AddCallerSkip(max(r.Skip, 0)))
		}
	}

	ce := zl.Check(levelMapper.Map(r.Level), r.Message)
	if ce == nil {
		return nil
	}

	ce.Write(h.attrsToZapFields(r.Attrs)...)

	return nil
}

// Enabled checks if the given log level is enabled.
func (h *zapHandler) Enabled(level handler.LogLevel) bool {
	return h.base.Enabled(level)
}

// WithAttrs returns a new logger with the provided keyValues added to the context.
func (h *zapHandler) WithAttrs(attrs []handler.Attr) handler.Handler {
	if len(attrs) == 0 {
		return h
	}

	return &zapHandler{
		base:        h.base,
		logger:      h.logger.With(h.attrsToZapFields(attrs)...),
		atomicLevel: h.atomicLevel,
	}
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (h *zapHandler) WithGroup(name string) handler.Handler {
	if name == "" {
		return h
	}

	return &zapHandler{
		base:        h.base.WithKeyPrefix(name),
		logger:      h.logger.With(zap.Namespace(name)),
		atomicLevel: h.atomicLevel,
	}
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (h *zapHandler) SetLevel(level handler.LogLevel) error {
	// Validate and store in base (atomic store inside BaseHandler)
	if err := h.base.SetLevel(level); err != nil {
		return err
	}

	// Reflect the authoritative base level into zap's atomic level
	h.atomicLevel.SetLevel(levelMapper.Map(h.base.Level()))

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

// WithCallerSkip returns a new handler with the caller skip permanently adjusted.
func (h *zapHandler) WithCallerSkip(skip int) (handler.Handler, error) {
	current := h.base.CallerSkip() - internalSkipFrames
	if skip == current {
		return h, nil
	}
	return h.WithCallerSkipDelta(skip - current)
}

// WithCallerSkipDelta returns a new handler with the caller skip permanently adjusted by delta.
func (h *zapHandler) WithCallerSkipDelta(delta int) (handler.Handler, error) {
	if delta == 0 {
		return h, nil
	}

	baseClone, err := h.base.WithCallerSkipDelta(delta)
	if err != nil {
		return h, err
	}

	return &zapHandler{
		base:        baseClone,
		logger:      h.logger.WithOptions(zap.AddCallerSkip(delta)),
		atomicLevel: h.atomicLevel,
	}, nil
}

// Sync flushes any buffered log entries.
func (h *zapHandler) Sync() error {
	return h.logger.Sync()
}

// attrsToZapFields transforms a slice of handler.Attr into zap.Fields.
func (h *zapHandler) attrsToZapFields(attrs []handler.Attr) []zap.Field {
	n := len(attrs)
	fields := make([]zap.Field, 0, n)

	// Compute prefix once
	prefix := ""
	if p := h.base.KeyPrefix(); p != "" {
		prefix = p + h.base.Separator()
	}

	for i := range n {
		a := attrs[i]
		fields = append(fields, attrToZapField(prefix+a.Key, a.Value))
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
	case bool:
		return zap.Bool(key, vv)
	case int:
		return zap.Int(key, vv)
	case int8:
		return zap.Int8(key, vv)
	case int16:
		return zap.Int16(key, vv)
	case int64:
		return zap.Int64(key, vv)
	case uint:
		return zap.Uint(key, vv)
	case uint8:
		return zap.Uint8(key, vv)
	case uint64:
		return zap.Uint64(key, vv)
	case float64:
		return zap.Float64(key, vv)
	case []byte:
		return zap.ByteString(key, vv)
	case time.Time:
		return zap.Time(key, vv)
	case error:
		return zap.Error(vv)
	default:
		return zap.Any(key, vv)
	}
}
