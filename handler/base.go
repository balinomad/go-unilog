package handler

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sync/atomic"

	"github.com/balinomad/go-atomicwriter"
)

// BaseOptions holds configuration common to most handlers.
type BaseOptions struct {
	Level        LogLevel  // Minimum log level
	Output       io.Writer // Output writer
	Format       string    // Handler-specific format string
	ValidFormats []string  // Optional: formats accepted by this handler (first is default)
	WithCaller   bool      // True if caller information should be included
	WithTrace    bool      // True if stack traces should be included
	CallerSkip   int       // User-specified caller skip frames
	Separator    string    // Key prefix separator (default: "_")
}

// BaseHandler provides shared functionality for handler implementations.
// Handlers that embed BaseHandler can use its optional helpers or ignore them
// in favor of their own optimized implementations.
type BaseHandler struct {
	out        *atomicwriter.AtomicWriter
	level      atomic.Int32
	format     string
	withCaller bool
	withTrace  bool
	callerSkip int

	// Optional prefix management for handlers without native support
	keyPrefix string
	separator string
}

// NewBaseHandler initializes shared resources.
func NewBaseHandler(opts BaseOptions) (*BaseHandler, error) {
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
		separator = "_" // Default separator
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

// AtomicWriter returns the underlying atomic writer.
// Handlers use this to get the thread-safe writer for backend initialization.
func (h *BaseHandler) AtomicWriter() *atomicwriter.AtomicWriter {
	return h.out
}

// Format returns the configured format string.
func (h *BaseHandler) Format() string {
	return h.format
}

// WithCaller returns whether caller information should be included.
func (h *BaseHandler) WithCaller() bool {
	return h.withCaller
}

// WithTrace returns whether stack traces should be included for error-level logs.
func (h *BaseHandler) WithTrace() bool {
	return h.withTrace
}

// CallerSkip returns the number of stack frames to skip for caller reporting.
// Handlers should add their internal frame count to this value.
func (h *BaseHandler) CallerSkip() int {
	return h.callerSkip
}

// Clone returns a shallow copy of BaseHandler for use in handler cloning.
// When a handler embeds BaseHandler, it should call this in its own Clone method.
func (h *BaseHandler) Clone() *BaseHandler {
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

// PrefixKey applies the current key prefix to a key string.
// Only use this if your handler lacks native group prefix support.
// Handlers with native support (zap, logrus, log15, slog) should ignore this.
//
// Performance note: This implementation is optimized for the common case
// where no prefix exists. See docs/HANDLERS.md for benchmark comparisons.
func (h *BaseHandler) PrefixKey(key string) string {
	if h.keyPrefix == "" {
		return key
	}
	return h.keyPrefix + h.separator + key
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

// SetSeparator changes the separator used for key prefixes (default: "_").
func (h *BaseHandler) SetSeparator(sep string) {
	h.separator = sep
}

// KeyPrefix returns the current key prefix.
func (h *BaseHandler) KeyPrefix() string {
	return h.keyPrefix
}

// Separator returns the current separator.
func (h *BaseHandler) Separator() string {
	return h.separator
}

// PrefixKeyMapper provides a flexible key prefixing strategy.
// Handlers can use this to switch between native and centralized prefix management.
type PrefixKeyMapper struct {
	useNative bool
	prefix    string
	separator string
	nativeMap func(key string) string // Handler's native implementation
}

// NewPrefixKeyMapper creates a new mapper with the given strategy.
// If useNative is true, nativeMap is used; otherwise, centralized logic applies.
func NewPrefixKeyMapper(useNative bool, prefix, separator string, nativeMap func(string) string) *PrefixKeyMapper {
	return &PrefixKeyMapper{
		useNative: useNative,
		prefix:    prefix,
		separator: separator,
		nativeMap: nativeMap,
	}
}

// Map applies the configured prefix strategy to the key.
func (m *PrefixKeyMapper) Map(key string) string {
	if m.useNative && m.nativeMap != nil {
		return m.nativeMap(key)
	}
	// Centralized implementation
	if m.prefix == "" {
		return key
	}
	return m.prefix + m.separator + key
}

// WithPrefix returns a new mapper with the prefix updated.
func (m *PrefixKeyMapper) WithPrefix(prefix string) *PrefixKeyMapper {
	newPrefix := prefix
	if m.prefix != "" {
		newPrefix = m.prefix + m.separator + prefix
	}
	return &PrefixKeyMapper{
		useNative: m.useNative,
		prefix:    newPrefix,
		separator: m.separator,
		nativeMap: m.nativeMap,
	}
}

// ProcessKeyValues applies prefix mapping to a slice of key-value pairs.
// Returns a new slice with prefixed keys. Only processes string keys.
func ProcessKeyValues(mapper *PrefixKeyMapper, keyValues []any) []any {
	if mapper == nil || len(keyValues) < 2 {
		return keyValues
	}

	processed := make([]any, len(keyValues))
	copy(processed, keyValues)

	for i := 0; i < len(processed)-1; i += 2 {
		if key, ok := processed[i].(string); ok {
			processed[i] = mapper.Map(key)
		} else {
			// Convert non-string keys to strings
			processed[i] = mapper.Map(fmt.Sprint(processed[i]))
		}
	}

	return processed
}
