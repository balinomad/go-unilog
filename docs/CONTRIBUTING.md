# Contributing a Handler

## Implementation Checklist

### 1. Create package under `handler/<name>/`

### 2. Implement `handler.Handler` interface

```go
type xyzHandler struct {
    base   *handler.BaseHandler  // Common functionality
    logger *xyz.Logger           // Backend logger

    // Handler-specific fields only (e.g., mapper strategy)
    mapper *handler.PrefixKeyMapper
}

func (h *xyzHandler) Handle(ctx context.Context, r *handler.Record) error {
    // Convert Record to backend format and write
}

func (h *xyzHandler) Enabled(level handler.LogLevel) bool {
    return h.base.Enabled(level)
}
```

### 3. Implement optional interfaces as supported (Chainer, Configurator, etc.)

### 4. Create `New()` constructor returning `handler.Handler`

Handler fully configures backend using BaseOptions.

```go
func New(opts ...XyzOption) (handler.Handler, error) {
    // 1. Parse handler-specific options
    o := &xyzOptions{
        BaseOptions: handler.BaseOptions{
            Level:  handler.InfoLevel,
            Output: os.Stderr,
        },
        // handler-specific defaults
    }

    for _, opt := range opts {
        if err := opt(o); err != nil {
            return nil, err
        }
    }

    // 2. Create BaseHandler (validates and normalizes)
    base, err := handler.NewBaseHandler(o.BaseOptions)
    if err != nil {
        return nil, err
    }

    // 3. Configure backend using base.AtomicWriter() and base.Level()
    encoder := createEncoder(base.Format()) // Use validated format
    backend := xyz.New(
        base.AtomicWriter(),                     // Thread-safe writer
        xyz.WithLevel(toXyzLevel(base.Level())), // Converted level
    )

    // 4. Assemble handler
    return &xyzHandler{
        base:   base,
        logger: backend,
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

### 5. Verify Compliance

Static assertions:

```go
var _ handler.Handler = (*xyzHandler)(nil)
var _ handler.Chainer = (*xyzHandler)(nil) // If Chainer is implemented
```

Use `ComplianceTest()` in test code:

```go
// Compliance tests in the test code
func TestCompliance(t *testing.T) {
    handler.ComplianceTest(t, func() (handler.Handler, error) {
        return xyzHandler(...)
    })
}
```

For additional test helpers, see [Test Helpers](#test-helpers)

## Practical Considerations

### Adjusting Caller Skip

Caller skip adjusts which stack frame is reported as the log call site. This is essential for wrapper libraries to report the correct caller location.

Use `WithCallerSkip()` to adjust reported caller location:

```go
// If wrapping logger in custom middleware:
logger, err := logger.WithCallerSkip(1) // Skip one additional frame
```

#### Calculation Formula

```
Reported Frame = ActualCaller + BaseSkip + UserSkip
```

**Example**:

```go
func myWrapper() {
    logger.Info(ctx, "message") // Reports this line
}

func myWrapperFixed() {
    handler, _ := handler.WithCallerSkipDelta(1)
    logger.Info(ctx, "message") // Reports caller of myWrapperFixed
}
```

## Test Helpers

```go
// Test helper to verify skip calculation
func verifySkipFrames(t *testing.T, logger unilog.Logger) {
    t.Helper()  // This frame should be skipped

    var buf bytes.Buffer
    logger.SetOutput(&buf)
    logger.Info(context.Background(), "test") // ← This line should be reported

    output := buf.String()
    if !strings.Contains(output, "handler_test.go:123") {
        t.Errorf("wrong caller: %s", output)
    }
}
```
## Example

See [`handler/slog/slog.go`](../handler/slog/slog.go) for reference implementation.
