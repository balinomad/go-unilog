# Handler Development Guide

This guide covers implementing custom handlers for unilog. For general contribution guidelines (PRs, code standards), see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Overview

A **handler** adapts a third-party logging library to the `handler.Handler` interface. It translates unilog's unified API into backend-specific calls.

**What handlers do:**
- Convert log levels (unilog → backend)
- Transform key-value pairs to backend format
- Manage backend lifecycle (initialization, output swapping)
- Optionally implement advanced interfaces (Chainer, Configurator, Syncer)

**What handlers don't do:**
- Implement logging logic (delegate to backend)
- Validate inputs (unilog core handles this)
- Manage context beyond passing it to backend

## Prerequisites

Before implementing a handler:

1. **Understand the backend**: Read the third-party logger's documentation
2. **Check feature support**: Does it support caller skip? Groups? Level changes?
3. **Review existing handlers**: Study `handler/slog` and `handler/zap` for patterns

Required knowledge:
- Go interfaces and type assertions
- Concurrency primitives (mutexes, atomics)
- The `handler.BaseHandler` helper API

## Implementation Checklist

### 1. Create Package Structure

```
handler/<name>/
├── <name>.go          # Handler implementation
├── <name>_test.go     # Compliance and integration tests
├── go.mod             # Module with backend dependency
├── go.sum
└── README.md          # Handler-specific usage guide
```

### 2. Define Options

Embed `handler.BaseOptions` to leverage common configuration:

```go
package myhandler

import "github.com/balinomad/go-unilog/handler"

type myHandlerOptions struct {
    base *handler.BaseOptions
    // Handler-specific options
    bufferSize int
}

type MyHandlerOption func(*myHandlerOptions) error

// Wrap BaseOptions helpers
func WithLevel(level handler.LogLevel) MyHandlerOption {
    return func(o *myHandlerOptions) error {
        return handler.WithLevel(level)(o.base)
    }
}

func WithOutput(w io.Writer) MyHandlerOption {
    return func(o *myHandlerOptions) error {
        return handler.WithOutput(w)(o.base)
    }
}

// Handler-specific option
func WithBufferSize(size int) MyHandlerOption {
    return func(o *myHandlerOptions) error {
        if size <= 0 {
            return errors.New("buffer size must be positive")
        }
        o.bufferSize = size
        return nil
    }
}
```

### 3. Implement Handler Structure

```go
type myHandler struct {
    base    *handler.BaseHandler  // Provides common functionality
    backend *mylogger.Logger      // The actual backend logger

    // Cached flags for lock-free hot path
    withCaller bool
    withTrace  bool
    callerSkip int
}

// Static assertion: verify interface compliance at compile time
var _ handler.Handler = (*myHandler)(nil)
```

### 4. Implement Constructor (New)

The constructor must:
- Parse options
- Create BaseHandler (validates input)
- Configure backend using BaseHandler state
- Return fully initialized handler

```go
func New(opts ...MyHandlerOption) (handler.Handler, error) {
    // Default options
    o := &myHandlerOptions{
        base: &handler.BaseOptions{
            Level:  handler.DefaultLevel,
            Output: os.Stderr,
            Format: "json",
            ValidFormats: []string{"json", "text"},
        },
        bufferSize: 4096,
    }

    // Apply options
    for _, opt := range opts {
        if err := opt(o); err != nil {
            return nil, err
        }
    }

    // Create BaseHandler (validates and normalizes)
    base, err := handler.NewBaseHandler(o.base)
    if err != nil {
        return nil, err
    }

    // Configure backend using validated base state
    backend := mylogger.New(
        mylogger.WithWriter(base.AtomicWriter()),
        mylogger.WithLevel(toBackendLevel(base.Level())),
        mylogger.WithBufferSize(o.bufferSize),
    )

    return &myHandler{
        base:       base,
        backend:    backend,
        withCaller: base.CallerEnabled(),
        withTrace:  base.TraceEnabled(),
        callerSkip: base.CallerSkip(),
    }, nil
}
```

**Anti-pattern (avoid):**

```go
// Don't do this - creates duplicate validation and state
func New(backend *xyz.Logger, level LogLevel, output io.Writer) (handler.Handler, error) {
    // Now we have two sources of truth for level and output
}
```

### 5. Implement Required Methods

#### Handle(context.Context, *Record) error

Core logging method - convert Record to backend format:

```go
func (h *myHandler) Handle(ctx context.Context, r *handler.Record) error {
    // Fast path: check level before allocations
    if !h.Enabled(r.Level) {
        return nil
    }

    // Convert unilog level to backend level
    level := h.levelMapper.Map(r.Level)

    // Build backend-specific log entry
    entry := h.backend.WithLevel(level).
        WithTime(r.Time).
        WithMessage(r.Message)

    // Add key-value pairs
    for i := 0; i < len(r.KeyValues)-1; i += 2 {
        key := fmt.Sprint(r.KeyValues[i])
        entry = entry.WithField(key, r.KeyValues[i+1])
    }

    // Handle caller location if enabled
    if h.withCaller && r.PC != 0 {
        // Backend doesn't support native skip, use PC
        frame := runtime.FuncForPC(r.PC)
        file, line := frame.FileLine(r.PC)
        entry = entry.WithField("source", fmt.Sprintf("%s:%d", file, line))
    }

    // Handle stack traces if enabled
    if h.withTrace && r.Level >= handler.ErrorLevel {
        entry = entry.WithField("stack", string(debug.Stack()))
    }

    // Delegate to backend
    entry.Write()

    return nil
}
```

#### Enabled(LogLevel) bool

Level check - must be lock-free:

```go
func (h *myHandler) Enabled(level handler.LogLevel) bool {
    return h.base.Enabled(level)
}
```

#### HandlerState() HandlerState

Expose handler state for introspection:

```go
func (h *myHandler) HandlerState() handler.HandlerState {
    return h.base
}
```

#### Features() HandlerFeatures

Declare supported features:

```go
func (h *myHandler) Features() handler.HandlerFeatures {
    return handler.NewHandlerFeatures(
        handler.FeatNativeCaller |    // Backend supports skip parameter
        handler.FeatNativeGroup |     // Backend supports grouping
        handler.FeatBufferedOutput |  // Implements Syncer
        handler.FeatDynamicLevel |    // Implements SetLevel
        handler.FeatDynamicOutput,    // Implements SetOutput
    )
}
```

### 6. Implement Optional Interfaces

#### Chainer (Recommended)

Enables `With()` and `WithGroup()`:

```go
var _ handler.Chainer = (*myHandler)(nil)

func (h *myHandler) WithAttrs(keyValues []any) handler.Chainer {
    if len(keyValues) < 2 {
        return h
    }

    // Shallow clone sharing mutable state
    clone := h.shallowClone()
    clone.backend = h.backend.WithFields(convertKeyValues(keyValues))

    return clone
}

func (h *myHandler) WithGroup(name string) handler.Chainer {
    if name == "" {
        return h
    }

    clone := h.shallowClone()
    clone.backend = h.backend.WithNamespace(name)

    return clone
}
```

#### AdvancedHandler (Optional)

Enables immutable configuration methods:

```go
var _ handler.AdvancedHandler = (*myHandler)(nil)

func (h *myHandler) WithLevel(level handler.LogLevel) handler.AdvancedHandler {
    newBase, err := h.base.WithLevel(level)
    if err != nil || newBase == h.base {
        return h
    }

    // Deep clone with new base
    return h.deepClone(newBase)
}

func (h *myHandler) WithCallerSkip(skip int) handler.AdvancedHandler {
    current := h.base.CallerSkip()
    if skip == current {
        return h
    }

    newBase, err := h.base.WithCallerSkip(skip)
    if err != nil {
        return h
    }

    return h.deepClone(newBase)
}

// Similar for WithOutput, WithCaller, WithTrace, WithCallerSkipDelta
```

#### Configurator (If Backend Supports)

Enables runtime reconfiguration:

```go
var _ handler.Configurator = (*myHandler)(nil)

func (h *myHandler) SetLevel(level handler.LogLevel) error {
    if err := h.base.SetLevel(level); err != nil {
        return err
    }

    // Update backend
    h.backend.SetMinLevel(toBackendLevel(level))

    return nil
}

func (h *myHandler) SetOutput(w io.Writer) error {
    return h.base.SetOutput(w)
}
```

#### Syncer (If Backend Buffers)

Flush buffered output:

```go
var _ handler.Syncer = (*myHandler)(nil)

func (h *myHandler) Sync() error {
    return h.backend.Flush()
}
```

### 7. Implement Clone Helpers

```go
// shallowClone for Chainer (shares mutable state)
func (h *myHandler) shallowClone() *myHandler {
    return &myHandler{
        base:       h.base,
        backend:    h.backend,
        withCaller: h.withCaller,
        withTrace:  h.withTrace,
        callerSkip: h.callerSkip,
    }
}

// deepClone for AdvancedHandler (independent state)
func (h *myHandler) deepClone(base *handler.BaseHandler) *myHandler {
    backend := mylogger.New(
        mylogger.WithWriter(base.AtomicWriter()),
        mylogger.WithLevel(toBackendLevel(base.Level())),
    )

    return &myHandler{
        base:       base,
        backend:    backend,
        withCaller: base.CallerEnabled(),
        withTrace:  base.TraceEnabled(),
        callerSkip: base.CallerSkip(),
    }
}
```

### 8. Level Mapping

Use `handler.LevelMapper` for consistent conversion:

```go
var levelMapper = handler.NewLevelMapper(
    mylogger.Trace,    // TraceLevel
    mylogger.Debug,    // DebugLevel
    mylogger.Info,     // InfoLevel
    mylogger.Warn,     // WarnLevel
    mylogger.Error,    // ErrorLevel
    mylogger.Error,    // CriticalLevel (if no native equivalent)
    mylogger.Fatal,    // FatalLevel
    mylogger.Panic,    // PanicLevel
)

func toBackendLevel(level handler.LogLevel) mylogger.Level {
    return levelMapper.Map(level)
}
```

## Testing

### Compliance Tests (Required)

Verify your handler meets interface contracts:

```go
func TestCompliance(t *testing.T) {
    handler.ComplianceTest(t, func() (handler.Handler, error) {
        return New(WithOutput(io.Discard))
    })
}
```

### Integration Tests (Recommended)

Verify backend integration:

```go
func TestHandle(t *testing.T) {
    var buf bytes.Buffer
    h, _ := New(WithOutput(&buf), WithFormat("json"))

    ctx := context.Background()
    r := &handler.Record{
        Time:      time.Now(),
        Level:     handler.InfoLevel,
        Message:   "test message",
        KeyValues: []any{"key", "value"},
    }

    if err := h.Handle(ctx, r); err != nil {
        t.Fatalf("Handle() failed: %v", err)
    }

    output := buf.String()
    if !strings.Contains(output, "test message") {
        t.Errorf("output missing message: %s", output)
    }
    if !strings.Contains(output, `"key":"value"`) {
        t.Errorf("output missing key-value: %s", output)
    }
}
```

### Interface Tests (If Implemented)

Test optional interfaces:

```go
func TestChainer(t *testing.T) {
    h, _ := New(WithOutput(io.Discard))

    h2 := h.WithAttrs([]any{"service", "api"})
    if h2 == h {
        t.Error("WithAttrs should return new instance")
    }

    h3 := h.WithGroup("database")
    if h3 == h {
        t.Error("WithGroup should return new instance")
    }
}

func TestConfigurator(t *testing.T) {
    h, _ := New(WithLevel(handler.InfoLevel), WithOutput(io.Discard))

    if err := h.SetLevel(handler.DebugLevel); err != nil {
        t.Fatalf("SetLevel() failed: %v", err)
    }

    if !h.Enabled(handler.DebugLevel) {
        t.Error("handler not enabled at DebugLevel after SetLevel")
    }
}
```

## Common Patterns

### Caller Skip Calculation

If backend supports native skip:

```go
func (h *myHandler) Handle(ctx context.Context, r *handler.Record) error {
    // Use Record.Skip for per-call adjustment
    if h.withCaller && r.Skip != 0 {
        h.backend.WithCallerSkip(r.Skip).Log(...)
    } else {
        h.backend.Log(...)
    }
    return nil
}
```

If backend doesn't support skip, use `Record.PC`:

```go
func (h *myHandler) Handle(ctx context.Context, r *handler.Record) error {
    if h.withCaller && r.PC != 0 {
        frame := runtime.FuncForPC(r.PC)
        file, line := frame.FileLine(r.PC)
        // Add to output
    }
    return nil
}
```

### Key Prefix Management

If backend lacks native grouping, use `BaseHandler.ApplyPrefix()`:

```go
func (h *myHandler) WithGroup(name string) handler.Chainer {
    clone := h.shallowClone()
    clone.base = h.base.WithKeyPrefix(name)
    return clone
}

func (h *myHandler) Handle(ctx context.Context, r *handler.Record) error {
    // Apply prefix to keys
    for i := 0; i < len(r.KeyValues)-1; i += 2 {
        key := h.base.ApplyPrefix(fmt.Sprint(r.KeyValues[i]))
        // Use prefixed key
    }
}
```

### Performance Optimization

**Hot path priorities:**
1. Level check before allocations
2. Cache flags (avoid lock on every log)
3. Stack-allocate small slices
4. Minimize interface conversions

```go
func (h *myHandler) Handle(ctx context.Context, r *handler.Record) error {
    // 1. Level check (lock-free)
    if !h.Enabled(r.Level) {
        return nil
    }

    // 2. Use cached flags (no mutex)
    if h.withCaller {
        // ...
    }

    // 3. Stack-allocate for common case
    const stackN = 8
    var stackFields [stackN]Field
    var fields []Field
    if len(r.KeyValues)/2 <= stackN {
        fields = stackFields[:0]
    } else {
        fields = make([]Field, 0, len(r.KeyValues)/2)
    }

    // 4. Minimize conversions
    // Convert once, reuse result

    return nil
}
```

---

## Pitfalls to Avoid

1. **Don't duplicate BaseHandler logic**: Use `base.Enabled()`, `base.AtomicWriter()`
2. **Don't ignore Record.PC/Skip**: Caller detection depends on these
3. **Don't mutate Record**: Handlers must be side-effect free on Record
4. **Don't block in Handle()**: Respect context cancellation, avoid slow operations
5. **Don't panic**: Return errors for unrecoverable failures only
6. **Don't assume key types**: Always convert to string (`fmt.Sprint(key)`)
7. **Don't skip compliance tests**: They catch contract violations early

---

## Performance Guidelines

### Allocation Budget (per log call)

| Handler Type | Target Allocations | Notes |
|--------------|-------------------|-------|
| Zero-alloc (zap, zerolog) | 0-2 | Only unavoidable allocations |
| Standard (slog, logrus) | 3-5 | Record + fields slice + backend |
| Simple (stdlog) | 3-7 | String building acceptable |

### Latency Budget (overhead vs direct backend)

- **Hot path (Handle)**: < 10ns added latency
- **Warm path (WithAttrs, WithGroup)**: < 200ns
- **Cold path (New, SetLevel)**: < 2μs

### Memory Usage

- Avoid retaining large slices across calls
- Pool allocations if handler processes high volume
- Consider backend's memory characteristics

---

## Examples

See existing handlers for reference implementations:

- **[handler/slog](../../handler/slog/)**: Standard library, native features
- **[handler/zap](../../handler/zap/)**: High-performance, full feature set
- **[handler/stdlog](../../handler/stdlog/)**: Minimal dependencies, simple

---

## Checklist Before PR

- [ ] Handler implements `handler.Handler` (static assertion)
- [ ] Constructor validates options via `BaseHandler`
- [ ] Level mapper defined for all 8 levels
- [ ] Compliance tests pass
- [ ] Integration tests cover Handle(), Enabled()
- [ ] Optional interfaces tested if implemented
- [ ] README.md created in handler directory
- [ ] Feature matrix updated in docs/HANDLERS.md
- [ ] No gocyclo violations (functions ≤ 15)
- [ ] No race conditions (verified with -race flag)

---

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bugs**: File an issue with reproducer
- **Design feedback**: Open an issue before starting major work

See [CONTRIBUTING.md](../CONTRIBUTING.md) for PR process and code standards.