package handler

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/balinomad/go-atomicwriter"
)

// StateFlag is a set of flags used to track handler state.
type StateFlag uint32

const (
	FlagCaller StateFlag = 1 << iota // Enable caller location reporting
	FlagTrace                        // Enable stack trace reporting for ERROR and above
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
		if len(o.ValidFormats) > 0 && !slices.Contains(o.ValidFormats, format) {
			return NewOptionApplyError("WithFormat", NewInvalidFormatError(format, o.ValidFormats))
		}
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
// All methods are thread-safe. Handlers should cache flag states at init
// for lock-free hot-path performance.
//
// Concurrency Model:
//
// BaseHandler uses two synchronization primitives:
//  1. sync.RWMutex (mu) protects format, callerSkip, keyPrefix, separator
//  2. atomic.Uint32/Int32 for flags and level (lock-free reads)
//
// Design rationale:
//   - Logging (hot path) requires lock-free flag checks
//   - Configuration changes (cold path) can tolerate mutex overhead
//   - Handlers should cache flag states at init for zero-lock logging
//
// Performance:
//   - Handlers SHOULD cache immutable config (format, separator) at initialization
//     to avoid RWMutex contention in hot path. See handler/slog for example.
//   - Mutable config (level, output) uses atomics/AtomicWriter for lock-free access.
//
// Mutability semantics:
//   - Set* methods mutate in-place (affect shared state)
//   - With* methods return new instances (immutable pattern)
//   - Clone() creates independent copy with separate mutex but shared AtomicWriter
//
// Example handler optimization:
//
//	type myHandler struct {
//	    base        *BaseHandler
//	    needsCaller bool // Cached at init
//	    format      string // Cached at init
//	}
//	func (h *myHandler) Handle(...) {
//	    if h.needsCaller { /* no lock */ }
//	}
//
// State Management:
//   - Shared state: level, output, format (modified via Set* methods)
//   - Independent state: keyPrefix, callerSkip (cloned via With* methods)
//
// Caller Detection:
//   - Handlers needing source location should use [github.com/balinomad/go-caller].
//   - See [github.com/balinomad/go-unilog/handler/stdlog] for an example.
type BaseHandler struct {
	mu         sync.RWMutex  // Protects format, callerSkip, keyPrefix, separator
	flags      atomic.Uint32 // StateFlag bitmask (lock-free)
	level      atomic.Int32  // LogLevel (lock-free for Enabled())
	out        *atomicwriter.AtomicWriter
	callerSkip int
	format     string
	keyPrefix  string
	separator  string
}

// maxKeyPrefixLength is the maximum total length of accumulated key prefixes.
// Prevents pathological cases with deep nesting or long key names.
// 10,000 characters should handle reasonable nesting (e.g., 100 levels * 100 chars each).
const maxKeyPrefixLength = 10000

// Ensure BaseHandler implements HandlerState
var _ HandlerState = (*BaseHandler)(nil)

// NewBaseHandler initializes a new BaseHandler.
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
		callerSkip: opts.CallerSkip,
		separator:  separator,
	}
	h.level.Store(int32(opts.Level))

	// Initialize flags
	var flags uint32
	if opts.WithCaller {
		flags |= uint32(FlagCaller)
	}
	if opts.WithTrace {
		flags |= uint32(FlagTrace)
	}
	h.flags.Store(flags)

	return h, nil
}

// --- Thread-Safe State Access ---

// Enabled reports whether the handler processes records at the given level.
func (h *BaseHandler) Enabled(level LogLevel) bool {
	return level >= LogLevel(h.level.Load())
}

// Level returns the current minimum log level.
func (h *BaseHandler) Level() LogLevel {
	return LogLevel(h.level.Load())
}

// Format returns the configured format string.
func (h *BaseHandler) Format() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.format
}

// CallerEnabled returns whether caller information should be included.
func (h *BaseHandler) CallerEnabled() bool {
	return h.HasFlag(FlagCaller)
}

// TraceEnabled returns whether stack traces should be included for error-level logs.
func (h *BaseHandler) TraceEnabled() bool {
	return h.HasFlag(FlagTrace)
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

// --- Flag Management (Lock-Free) ---

// HasFlag checks if flag is set (lock-free).
func (h *BaseHandler) HasFlag(flag StateFlag) bool {
	return h.flags.Load()&uint32(flag) != 0
}

// SetFlag atomically sets or clears a flag.
// Affects all instances sharing this base.
func (h *BaseHandler) SetFlag(flag StateFlag, enabled bool) {
	for {
		old := h.flags.Load()
		new := old
		if enabled {
			new |= uint32(flag)
		} else {
			new &^= uint32(flag)
		}
		if h.flags.CompareAndSwap(old, new) {
			return
		}
	}
}

// --- Mutable Setters (Affect Shared State) ---

// SetLevel changes the minimum level of logs that will be processed.
// Affects all instances sharing this base.
func (h *BaseHandler) SetLevel(level LogLevel) error {
	if err := ValidateLogLevel(level); err != nil {
		return err
	}

	h.level.Store(int32(level))

	return nil
}

// SetOutput changes the destination for log output.
// Affects all instances sharing this base.
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
// Affects all instances sharing this base.
func (h *BaseHandler) SetCallerSkip(skip int) error {
	if skip < 0 {
		return ErrInvalidSourceSkip
	}

	h.mu.Lock()
	h.callerSkip = skip
	h.mu.Unlock()

	return nil
}

// --- Immutable Builders (Return New Instances) ---

// Clone returns a shallow copy of BaseHandler with independent mutex.
// The new instance shares the AtomicWriter but has separate state locks.
// This means SetOutput() on the clone affects the original's output destination.
// For fully independent handlers, create separate handler instances with different writers.
func (h *BaseHandler) Clone() *BaseHandler {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clone := &BaseHandler{
		out:        h.out, // Shared writer - SetOutput() affects original
		format:     h.format,
		callerSkip: h.callerSkip,
		keyPrefix:  h.keyPrefix,
		separator:  h.separator,
	}
	clone.level.Store(h.level.Load())
	clone.flags.Store(h.flags.Load())

	return clone
}

// WithLevel returns a shallow copy of BaseHandler with level set.
// If the level is already set, returns the original instance.
func (h *BaseHandler) WithLevel(level LogLevel) (*BaseHandler, error) {
	if err := ValidateLogLevel(level); err != nil {
		return nil, err
	}

	if LogLevel(h.level.Load()) == level {
		return h, nil
	}

	clone := h.Clone()
	clone.level.Store(int32(level))

	return clone, nil
}

// WithCaller returns a shallow copy of BaseHandler with caller flag set.
// If the caller flag is already set, returns the original instance.
func (h *BaseHandler) WithCaller(enabled bool) *BaseHandler {
	if h.HasFlag(FlagCaller) == enabled {
		return h
	}

	clone := h.Clone()
	clone.SetFlag(FlagCaller, enabled)

	return clone
}

// WithTrace returns a shallow copy of BaseHandler with trace flag set.
// If the trace flag is already set, returns the original instance.
func (h *BaseHandler) WithTrace(enabled bool) *BaseHandler {
	if h.HasFlag(FlagTrace) == enabled {
		return h
	}

	clone := h.Clone()
	clone.SetFlag(FlagTrace, enabled)

	return clone
}

// WithKeyPrefix returns a shallow copy of BaseHandler with the given prefix applied.
// This supports WithGroup for handlers without native prefix support.
// Returns error if total prefix length exceeds maxKeyPrefixLength.
func (h *BaseHandler) WithKeyPrefix(prefix string) (*BaseHandler, error) {
	h.mu.RLock()
	currentPrefix := h.keyPrefix
	sep := h.separator
	h.mu.RUnlock()

	// Calculate new prefix length
	newPrefixLen := len(prefix)
	if currentPrefix != "" {
		newPrefixLen += len(currentPrefix) + len(sep)
	}

	if newPrefixLen > maxKeyPrefixLength {
		return nil, fmt.Errorf("key prefix length (%d) exceeds maximum (%d characters)", newPrefixLen, maxKeyPrefixLength)
	}

	clone := h.Clone()

	if clone.keyPrefix == "" {
		clone.keyPrefix = prefix
	} else {
		clone.keyPrefix = clone.keyPrefix + clone.separator + prefix
	}

	return clone, nil
}

// WithCallerSkip returns a shallow copy of BaseHandler with updated caller skip.
// If the skip is already set, returns the original instance.
func (h *BaseHandler) WithCallerSkip(skip int) (*BaseHandler, error) {
	if skip < 0 {
		return nil, ErrInvalidSourceSkip
	}

	h.mu.RLock()
	current := h.callerSkip
	h.mu.RUnlock()

	if current == skip {
		return h, nil
	}

	clone := h.Clone()
	clone.callerSkip = skip

	return clone, nil
}

// WithCallerSkipDelta returns a shallow copy of BaseHandler with caller skip adjusted by delta.
// If delta is zero , returns the original instance.
// If the new skip is negative, it returns an error.
func (h *BaseHandler) WithCallerSkipDelta(delta int) (*BaseHandler, error) {
	if delta == 0 {
		return h, nil
	}

	h.mu.RLock()
	skip := h.callerSkip + delta
	h.mu.RUnlock()
	if skip < 0 {
		return nil, ErrInvalidSourceSkip
	}

	return h.WithCallerSkip(skip)
}

// WithOutput returns a new BaseHandler with output set.
// Error is returned for nil writer or if AtomicWriter creation fails.
// Current implementation only fails on nil writer, but error return is kept
// for future extensibility (e.g., writer validation, resource acquisition).
func (h *BaseHandler) WithOutput(w io.Writer) (*BaseHandler, error) {
	if w == nil {
		return nil, ErrNilWriter
	}

	aw, err := atomicwriter.NewAtomicWriter(w)
	if err != nil {
		return nil, NewAtomicWriterError(err)
	}

	clone := h.Clone()
	clone.out = aw

	return clone, nil
}
