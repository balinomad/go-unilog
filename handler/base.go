package handler

import (
	"errors"
	"io"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/balinomad/go-atomicwriter"
)

// DefaultKeySeparator is the default separator for group key prefixes.
const DefaultKeySeparator = "_"

// BaseOptions holds configuration common to most handlers.
type BaseOptions struct {
	Level  LogLevel  // Minimum log level
	Output io.Writer // Output writer

	// Format specifies the output format (e.g., "json", "text").
	// Optional if ValidFormats is empty (handler doesn't support format selection).
	// When ValidFormats is provided but Format is empty, defaults to ValidFormats[0].
	Format string

	// ValidFormats lists accepted format strings for this handler.
	// If provided, Format must be one of these values or empty (uses first as default).
	// Leave empty if handler doesn't support format configuration.
	ValidFormats []string

	WithCaller bool   // True if caller information should be included
	WithTrace  bool   // True if stack traces should be included
	CallerSkip int    // User-specified caller skip frames
	Separator  string // Key prefix separator (default: "_")
}

// BaseOption configures the BaseHandler.
type BaseOption func(*BaseOptions) error

// WithLevel sets the minimum log level.
func WithLevel(level LogLevel) BaseOption {
	return func(o *BaseOptions) error {
		if err := ValidateLogLevel(level); err != nil {
			return NewOptionApplyError("WithLevel", err)
		}
		o.Level = level
		return nil
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) BaseOption {
	return func(o *BaseOptions) error {
		if w == nil {
			return NewOptionApplyError("WithOutput", ErrNilWriter)
		}
		o.Output = w
		return nil
	}
}

// WithFormat sets the output format.
func WithFormat(format string) BaseOption {
	return func(o *BaseOptions) error {
		o.Format = format
		return nil
	}
}

// WithSeparator sets the separator for group key prefixes.
func WithSeparator(separator string) BaseOption {
	return func(o *BaseOptions) error {
		o.Separator = separator
		return nil
	}
}

// WithCaller enables or disables source location reporting.
// If enabled, the handler will include the source location of the log
// call site in the log record. This can be useful for debugging, but may
// incur a performance hit due to the additional stack frame analysis.
// The default value is false.
func WithCaller(enabled bool) BaseOption {
	return func(o *BaseOptions) error {
		o.WithCaller = enabled
		return nil
	}
}

// WithTrace enabless or disables stack traces for ERROR and above.
// If enabled, the handler will include the stack trace of the log
// call site in the log record. This can be useful for debugging, but may
// incur a performance hit due to the additional stack frame analysis.
// The default value is false.
func WithTrace(enabled bool) BaseOption {
	return func(o *BaseOptions) error {
		o.WithTrace = enabled
		return nil
	}
}

// BaseHandler provides shared functionality for handler implementations.
// Handlers that embed BaseHandler can use its optional helpers or ignore them
// in favor of their own optimized implementations.
//
// State Management:
//   - Shared state: level, output, format (modified via Set* methods)
//   - Independent state: keyPrefix, callerSkip (cloned via With* methods)
//
// Setters (SetLevel, SetOutput) affect all clones sharing this handler.
// Builders (WithKeyPrefix, Clone) return new instances with independent state.
//
// Caller Detection:
//   - Handlers needing source location should use [github.com/balinomad/go-caller].
//   - See [github.com/balinomad/go-unilog/handler/stdlog] for an example.
type BaseHandler struct {
	out        *atomicwriter.AtomicWriter
	level      atomic.Int32
	format     string
	withCaller bool
	withTrace  bool
	callerSkip int
	keyPrefix  string
	separator  string
	mu         sync.RWMutex // Protects all fields below
}

// Ensure BaseHandler implements HandlerState
var _ HandlerState = (*BaseHandler)(nil)

// NewBaseHandler initializes shared resources.
func NewBaseHandler(opts *BaseOptions) (*BaseHandler, error) {
	if opts.Output == nil {
		return nil, NewAtomicWriterError(errors.New("output writer is required"))
	}

	// Validate format if ValidFormats provided
	if len(opts.ValidFormats) > 0 {
		if opts.Format != "" {
			if !slices.Contains(opts.ValidFormats, opts.Format) {
				return nil, NewInvalidFormatError(opts.Format, opts.ValidFormats)
			}
		} else {
			opts.Format = opts.ValidFormats[0]
		}
	}

	aw, err := atomicwriter.NewAtomicWriter(opts.Output)
	if err != nil {
		return nil, NewAtomicWriterError(err)
	}

	separator := opts.Separator
	if separator == "" {
		separator = DefaultKeySeparator
	}

	h := &BaseHandler{
		out:        aw,
		format:     opts.Format,
		withCaller: opts.WithCaller,
		withTrace:  opts.WithTrace,
		callerSkip: opts.CallerSkip,
		separator:  separator,
	}
	h.level.Store(int32(opts.Level))

	return h, nil
}

// Enabled reports whether the handler processes records at the given level.
func (h *BaseHandler) Enabled(level LogLevel) bool {
	return level >= LogLevel(h.level.Load())
}

// HandlerState returns an immutable HandlerState that exposes handler state.
func (h *BaseHandler) HandlerState() HandlerState {
	return h
}

// Level returns the current minimum log level.
func (h *BaseHandler) Level() LogLevel {
	return LogLevel(h.level.Load())
}

// Format returns the configured format string.
func (h *BaseHandler) Format() string {
	return h.format
}

// CallerEnabled returns whether caller information should be included.
func (h *BaseHandler) CallerEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.withCaller
}

// TraceEnabled returns whether stack traces should be included for error-level logs.
func (h *BaseHandler) TraceEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.withTrace
}

// CallerSkip returns the number of stack frames to skip for caller reporting.
// Handlers should add their internal skip constant to this value.
//
// Example:
//
//	totalSkip := myHandler.internalSkipFrames + h.base.CallerSkip() + dynSkip
func (h *BaseHandler) CallerSkip() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.callerSkip
}

// KeyPrefix returns the current key prefix.
func (h *BaseHandler) KeyPrefix() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.keyPrefix
}

// Separator returns the current separator.
func (h *BaseHandler) Separator() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.separator
}

// AtomicWriter returns the underlying atomic writer.
// Handlers use this to get the thread-safe writer for backend initialization.
func (h *BaseHandler) AtomicWriter() *atomicwriter.AtomicWriter {
	return h.out
}

// WithKeyPrefix returns a copy of BaseHandler with the given prefix applied.
// This supports WithGroup for handlers without native prefix support.
func (h *BaseHandler) WithKeyPrefix(prefix string) *BaseHandler {
	clone := h.Clone()
	if clone.keyPrefix == "" {
		clone.keyPrefix = prefix
	} else {
		clone.keyPrefix = clone.keyPrefix + clone.separator + prefix
	}
	return clone
}

// WithCallerSkip returns a new handler with updated caller skip.
func (h *BaseHandler) WithCallerSkip(skip int) (*BaseHandler, error) {
	if skip < 0 {
		return nil, ErrInvalidSourceSkip
	}

	clone := h.Clone()
	clone.callerSkip = skip

	return clone, nil
}

// WithCallerSkipDelta returns a new handler with caller skip adjusted by delta.
func (h *BaseHandler) WithCallerSkipDelta(delta int) (*BaseHandler, error) {
	skip := h.callerSkip + delta
	if skip < 0 {
		return nil, ErrInvalidSourceSkip
	}

	return h.WithCallerSkip(skip)
}

// Clone returns a shallow copy of BaseHandler for use in handler cloning.
// When a handler embeds BaseHandler, it should call this in its own Clone method.
func (h *BaseHandler) Clone() *BaseHandler {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clone := &BaseHandler{
		out:        h.out,
		format:     h.format,
		withCaller: h.withCaller,
		withTrace:  h.withTrace,
		callerSkip: h.callerSkip,
		keyPrefix:  h.keyPrefix,
		separator:  h.separator,
	}
	clone.level.Store(h.level.Load())

	return clone
}

// SetLevel changes the minimum level of logs that will be processed.
func (h *BaseHandler) SetLevel(level LogLevel) error {
	if err := ValidateLogLevel(level); err != nil {
		return err
	}

	h.level.Store(int32(level))

	return nil
}

// SetOutput changes the destination for log output.
func (h *BaseHandler) SetOutput(w io.Writer) error {
	if w == nil {
		return ErrNilWriter
	}

	if err := h.out.Swap(w); err != nil {
		return NewAtomicWriterError(err)
	}

	return nil
}

// SetCallerSkip changes the caller skip value.
// This modifies the handler in place. Use WithCallerSkip for immutable variant.
func (h *BaseHandler) SetCallerSkip(skip int) error {
	if skip < 0 {
		return ErrInvalidSourceSkip
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.callerSkip = skip

	return nil
}

// SetSeparator changes the separator used for key prefixes (default: "_").
func (h *BaseHandler) SetSeparator(sep string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.separator = sep
}

// ApplyPrefix applies the current key prefix to a key string.
// Only use this if your handler lacks native group prefix support.
// Handlers with native support (zap, logrus, log15, slog) should ignore this.
//
// Performance note: This implementation is optimized for the common case
// where no prefix exists. See docs/HANDLERS.md for benchmark comparisons.
func (h *BaseHandler) ApplyPrefix(key string) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.keyPrefix == "" {
		return key
	}
	return h.keyPrefix + h.separator + key
}
