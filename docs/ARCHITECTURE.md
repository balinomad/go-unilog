# Architecture

This document describes unilog's system design, component responsibilities, and key design decisions.

## System Overview

unilog is a **logging abstraction layer** that decouples application code from specific logging implementations. It provides:

1. **Unified interface**: Single `Logger` API for all backends
2. **Pluggable handlers**: Adapters for third-party loggers
3. **Feature normalization**: Consistent behavior across backends
4. **Zero-cost abstraction**: Minimal overhead vs direct backend use

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Code                         │
│               (business logic, request handlers)                │
└────────────────────────────────┬────────────────────────────────┘
                                 │ uses
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                          unilog.Logger                          │
│           (interface: Info, Error, With, WithGroup)             │
├─────────────────────────────────────────────────────────────────┤
│                      logger implementation                      │
│          (wraps handler.Handler, manages caller skip)           │
└────────────────────────────────┬────────────────────────────────┘
                                 │ delegates to
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                         handler.Handler                         │
│               (interface: Handle, Enabled, etc.)                │
├─────────────────────────────────────────────────────────────────┤
│                Handler Implementation (adapter)                 │
│         (e.g., slogHandler, zapHandler, stdlogHandler)          │
├─────────────────────────────────────────────────────────────────┤
│                       handler.BaseHandler                       │
│       (shared functionality: level, output, caller skip)        │
└────────────────────────────────┬────────────────────────────────┘
                                 │ writes to
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Backend Logger Library                      │
│               (slog, zap, logrus, zerolog, etc.)                │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Logger Interface (`unilog.Logger`)

**Purpose**: User-facing API for structured logging

**Responsibilities**:
- Provide convenience methods (Info, Error, Debug, etc.)
- Support attribute chaining (With, WithGroup)
- Accept context.Context for cancellation awareness

**Key Design**:
```go
type Logger interface {
    Log(ctx context.Context, level LogLevel, msg string, keyValues ...any)
    Enabled(level LogLevel) bool
    With(keyValues ...any) Logger
    WithGroup(name string) Logger

    // Convenience methods
    Info(ctx context.Context, msg string, keyValues ...any)
    Error(ctx context.Context, msg string, keyValues ...any)
    // ... etc for all levels
}
```

**Immutability**: `With()` and `WithGroup()` return new instances

### 2. Logger Implementation (`logger` struct)

**Purpose**: Bridge between user API and handler interface

**Responsibilities**:
- Translate convenience methods to `Handler.Handle()` calls
- Manage caller skip calculation
- Respect context cancellation
- Cache handler capabilities for performance

**Key State**:
```go
type logger struct {
    handlerEntry              // cached handler components
    skip         int          // current caller skip
    mu           sync.RWMutex // protects shared state
}

type handlerEntry struct {
    h         handler.Handler         // core handler
    ch        handler.Chainer         // if supported
    adv       handler.AdvancedHandler // if supported
    cf        handler.MutableConfig   // if supported
    snc       handler.Syncer          // if supported
    state     handler.HandlerState    // handler state snapshot
    needsPC   bool                    // capture PC for caller?
    needsSkip bool                    // pass skip to handler?
    skip      int                     // current skip offset
}
```

**Caller Detection Logic**:
- **needsPC = true**: Handler lacks native caller skip, capture PC via `runtime.Callers()`
- **needsSkip = true**: Handler supports native skip, pass `Record.Skip`

### 3. Handler Interface (`handler.Handler`)

**Purpose**: Adapter contract for backend loggers

**Responsibilities**:
- Process log records (`Handle(ctx, Record)`)
- Report enabled levels (`Enabled(level)`)
- Expose handler state (`HandlerState()`)
- Declare supported features (`Features()`)

**Core Interface**:
```go
type Handler interface {
    Handle(ctx context.Context, record *Record) error
    Enabled(level LogLevel) bool
    HandlerState() HandlerState
    Features() HandlerFeatures
}
```

**Record Structure**:
```go
type Record struct {
    Time      time.Time  // Log timestamp
    Level     LogLevel   // Log level
    Message   string     // Log message
    KeyValues []any      // Key-value pairs (alternating)
    PC        uintptr    // Program counter (0 if unavailable)
    Skip      int        // Stack frames to skip (0 if not used)
}
```

**Feature Declaration**:
```go
type HandlerFeatures struct {
    features Feature
}

const (
    FeatNativeCaller       // Backend supports native caller skip
    FeatNativeGroup        // Backend supports native grouping
    FeatBufferedOutput     // Implements Syncer
    FeatContextPropagation // Passes context to backend
    FeatDynamicLevel       // Supports SetLevel
    FeatDynamicOutput      // Supports SetOutput
    FeatZeroAlloc          // Zero-allocation hot path
)
```

### 4. Optional Handler Interfaces

#### Chainer

**Purpose**: Enable attribute chaining (With, WithGroup)

**Mutability**: Methods return new handlers sharing mutable state (level, output)

```go
type Chainer interface {
    Handler
    WithAttrs(keyValues []any) Chainer
    WithGroup(name string) Chainer
}
```

**Example**: `logger.With("service", "api")` → handler.WithAttrs(["service", "api"])

#### AdvancedHandler

**Purpose**: Immutable configuration methods

**Mutability**: Methods return fully independent handlers

```go
type AdvancedHandler interface {
    Handler
    WithLevel(level LogLevel) AdvancedHandler
    WithOutput(w io.Writer) AdvancedHandler
    WithCallerSkip(skip int) AdvancedHandler
    WithCallerSkipDelta(delta int) AdvancedHandler
    WithCaller(enabled bool) AdvancedHandler
    WithTrace(enabled bool) AdvancedHandler
}
```

**Use case**: Module-specific log levels without affecting parent logger

#### MutableConfig

**Purpose**: Runtime reconfiguration (mutable)

```go
type MutableConfig interface {
    SetLevel(level LogLevel) error
    SetOutput(w io.Writer) error
}
```

**Use case**: Change log level in production without restart

#### Syncer

**Purpose**: Flush buffered output

```go
type Syncer interface {
    Sync() error
}
```

**Use case**: Ensure logs are written before shutdown (zap)

### 5. BaseHandler (`handler.BaseHandler`)

**Purpose**: Shared functionality for all handlers

**Responsibilities**:
- Validate configuration (level, output, format)
- Provide thread-safe atomic writer
- Track caller skip, key prefix, separator
- Offer helper methods for handlers

**Key Features**:
```go
type BaseHandler struct {
    mu         sync.RWMutex          // Protects mutable state
    flags      atomic.Uint32         // Caller/trace flags
    level      atomic.Int32          // Current log level
    out        *atomicwriter.AtomicWriter
    callerSkip int
    format     string
    keyPrefix  string
    separator  string
}
```

**Concurrency Model**:
- **Lock-free reads**: `Enabled()`, `HasFlag()` use atomics
- **Mutex-protected writes**: `SetLevel()`, `SetOutput()` use mutex
- **Cloning**: `With*()` methods return new instances with separate locks

**Design Rationale**: Hot path (logging) must be lock-free; cold path (configuration) can use mutex

## Data Flow

### Simple Log Call

```
1. Application: logger.Info(ctx, "msg", "key", "val")
   │
2. logger.Info() → logger.log(InfoLevel, "msg", 0, ["key", "val"])
   │
3. Check: ctx.Err() != nil? → skip
   │
4. Build Record:
   - Time: time.Now()
   - Level: InfoLevel
   - Message: "msg"
   - KeyValues: ["key", "val"]
   - PC: runtime.Callers(skip) [if needed]
   - Skip: calculated [if needed]
   │
5. handler.Handle(ctx, record)
   │
6. Handler checks: Enabled(InfoLevel)? → yes
   │
7. Handler converts:
   - Level: InfoLevel → backend level
   - KeyValues: to backend format
   - PC/Skip: to caller location
   │
8. Backend: backendLogger.Log(level, msg, fields)
   │
9. Output: written to configured destination
```

### Attribute Chaining

```
1. Application: logger.With("service", "api").Info(ctx, "msg")
   │
2. logger.With() → handler.WithAttrs(["service", "api"])
   │
3. Handler: returns new Chainer with attributes
   │
4. Application: chainedLogger.Info(ctx, "msg")
   │
5. logger.log() → handler.Handle(record)
   │
6. Handler: merges chained attrs + record keyValues
   │
7. Backend: logs with all attributes
```

### Group Nesting

```
1. logger.WithGroup("db").WithGroup("conn").Info(ctx, "query", "sql", "SELECT")
   │
2. First WithGroup("db"):
   - slog: handler.WithGroup("db") → native
   - stdlog: keyPrefix = "db"
   │
3. Second WithGroup("conn"):
   - slog: handler.WithGroup("conn") → native
   - stdlog: keyPrefix = "db_conn"
   │
4. Log call:
   - slog: {"db":{"conn":{"msg":"query","sql":"SELECT"}}}
   - stdlog: msg="query" db_conn_sql="SELECT"
```

## Key Design Decisions

### 1. Handler-Based Architecture

**Decision**: Use adapter pattern with `handler.Handler` interface

**Rationale**:
- Decouple user API from backend details
- Enable consistent behavior across backends
- Allow feature normalization (e.g., emulate caller skip)
- Support testing via mock handlers

**Trade-off**: Extra indirection vs direct backend calls (+5-10ns per log)

### 2. Record-Based Handle Method

**Decision**: `Handle(ctx, *Record)` instead of `Handle(ctx, level, msg, ...any)`

**Rationale**:
- Single allocation for all log data
- Extensible (add fields without breaking API)
- Clean separation of data (Record) and behavior (Handler)

**Trade-off**: Struct allocation per log call

### 3. Dual Caller Detection

**Decision**: Support both PC-based and skip-based caller detection

**Rationale**:
- Some backends support native skip (zap: `AddCallerSkip`)
- Some require explicit PC (slog: `NewRecord(pc)`)
- Others lack native support (stdlog: resolve from PC)

**Implementation**:
```go
// Handler declares capability
func (h *zapHandler) Features() HandlerFeatures {
    return NewHandlerFeatures(FeatNativeCaller)
}

// Logger checks feature
if features.Supports(FeatNativeCaller) {
    record.Skip = calculatedSkip
} else {
    runtime.Callers(skip, pcs[:])
    record.PC = pcs[0]
}
```

### 4. Immutable vs Mutable APIs

**Decision**: Provide both immutable (With*) and mutable (Set*) APIs

**Rationale**:
- **Immutable**: Safe request-scoped logging, no shared state bugs
- **Mutable**: Efficient runtime reconfiguration (change level without restart)

**Implementation**:
- `With*()` methods → return new instance, independent state
- `Set*()` methods → mutate in place, affect all derived loggers

**Example**:
```go
// Immutable: independent loggers
dbLogger := logger.WithLevel(DebugLevel)
apiLogger := logger.WithLevel(InfoLevel)
dbLogger.SetLevel(WarnLevel) // Only affects dbLogger

// Mutable: shared state
parent := logger.With("service", "api")
child := parent.With("endpoint", "/users")
parent.SetLevel(DebugLevel) // Affects child too
```

### 5. BaseHandler Helper

**Decision**: Provide `BaseHandler` for common functionality

**Rationale**:
- Reduce code duplication across handlers
- Ensure consistent validation (level, output, format)
- Simplify handler implementation
- Provide tested, thread-safe primitives

**Usage Pattern**:
```go
type myHandler struct {
    base *handler.BaseHandler
    backend *mylogger.Logger
}

func (h *myHandler) Enabled(level LogLevel) bool {
    return h.base.Enabled(level) // Delegate to BaseHandler
}
```

### 6. Context Cancellation Respect

**Decision**: Skip logging if `ctx.Err() != nil`

**Rationale**:
- Avoid wasting resources on canceled requests
- Consistent with Go context best practices
- Negligible overhead (single atomic load)

**Implementation**:
```go
func (l *logger) log(...) {
    if ctx != nil && ctx.Err() != nil {
        return // Skip if canceled
    }
    // ... build record and log
}
```

### 7. Feature-Based Capability Detection

**Decision**: Use feature flags instead of interface type assertions in hot path

**Rationale**:
- Avoid repeated type assertions per log call
- Cache capabilities at logger creation
- Enable compile-time feature documentation

**Pattern**:
```go
// At logger creation (cold path)
features := handler.Features()
needsPC := !features.Supports(FeatNativeCaller)

// At log call (hot path)
if needsPC {
    runtime.Callers(...) // No type assertion needed
}
```

## Performance Considerations

### Hot Path Optimization

**Goal**: Minimize overhead in `logger.log()` → `handler.Handle()`

**Strategies**:
1. **Early returns**: Check `Enabled()`, `ctx.Err()` before allocations
2. **Cache flags**: Store `needsPC`, `needsSkip` at creation, avoid locks
3. **Lock-free reads**: Use atomics for level checks
4. **Stack allocations**: Small attribute slices on stack
5. **Avoid conversions**: Minimize `interface{}` boxing

**Measured Overhead** (preliminary):
- slog: ~5-10ns vs direct `slog.Info()`
- zap: ~3-7ns vs direct `zap.Info()`
- stdlog: ~8-12ns vs direct `log.Print()`

### Memory Allocation

**Target**: 1-2 allocations per log call

**Breakdown**:
1. **Record struct**: 1 allocation (required)
2. **KeyValues slice**: 0 if ≤ N attrs (stack), 1 if > N (heap)
3. **Backend conversion**: varies by handler

**Optimization Techniques**:
- Stack-allocate common case (≤ 8 attributes)
- Reuse Record objects (future: sync.Pool)
- Minimize string operations in hot path

### Concurrency

**Lock-Free Operations**:
- `Enabled()`: atomic level check
- `HasFlag()`: atomic flag read
- Level mapping: read-only lookup table

**Mutex-Protected Operations**:
- `SetLevel()`: infrequent, acceptable overhead
- `SetOutput()`: rare, rebuilds handler if needed
- `With*()` cloning: creates new mutex, no contention

## Extension Points

### Adding New Handlers

1. Implement `handler.Handler` interface
2. Optionally implement `Chainer`, `AdvancedHandler`, `MutableConfig`, `Syncer`
3. Use `BaseHandler` for common functionality
4. Declare features via `Features()`
5. Write compliance tests

See [HANDLER_DEVELOPMENT.md](HANDLER_DEVELOPMENT.md) for detailed guide.

### Adding New Features

**Process**:
1. Propose in GitHub Discussion/Issue
2. Define interface extension (if needed)
3. Update `HandlerFeatures` with new flag
4. Implement in handlers that support it
5. Update documentation and tests

**Example**: Adding sampling support
```go
// 1. Define interface
type Sampler interface {
    WithSampling(rate float64) Sampler
}

// 2. Add feature flag
const FeatSampling Feature = 1 << 8

// 3. Handlers implement
func (h *zapHandler) WithSampling(rate float64) Sampler {
    // Use zap's sampling
}

// 4. Feature detection
if sampler, ok := handler.(Sampler); ok {
    handler = sampler.WithSampling(0.1)
}
```

## Testing Strategy

### Unit Tests

- **Logger**: Verify level methods call `log()` correctly
- **Handler**: Verify `Handle()` converts Record to backend format
- **BaseHandler**: Verify level/output/flag management

### Integration Tests

- **End-to-end**: Logger → Handler → Backend → Output
- **Interface compliance**: `handler.ComplianceTest()`
- **Concurrency**: Race detector, concurrent log calls

### Benchmark Tests

- **Hot path**: `logger.Info()` throughput
- **Allocation**: Allocations per log call
- **Contention**: Multiple goroutines logging

## Security Considerations

### Input Validation

- **No user input in format strings**: Only use `msg` parameter as format
- **Key type safety**: Always convert keys to strings
- **Value sanitization**: Handlers may sanitize sensitive values

### Context Cancellation

- **DoS prevention**: Canceled requests don't waste log processing
- **Resource cleanup**: `ctx.Done()` check before expensive operations

### Error Handling

- **No panics**: Handlers return errors, don't panic
- **Graceful degradation**: Log errors to stderr if handler fails

## Versioning and Stability

### API Stability

- **Core interfaces**: Stable, semantic versioning
- **Handler interface**: Stable, additive changes only
- **Optional interfaces**: Can add, cannot remove methods

### Compatibility Policy

See [COMPATIBILITY.md](COMPATIBILITY.md) for:
- Version support policy
- Deprecation process
- Breaking change guidelines

## Future Directions

### Planned Features (Not Yet Implemented)

1. **Structured Fields**: Strongly-typed field types (Duration, Error, Stringer)
2. **Log Sampling**: Reduce volume by sampling at configurable rates
3. **Context Extraction**: Automatic trace ID propagation from context
4. **Handler Middleware**: Composable handler wrappers (rate limiting, filtering)
5. **Async Logging**: Background goroutine for high-throughput scenarios

### Research Areas

1. **Zero-copy logging**: Reduce allocations via buffer pooling
2. **Compile-time optimization**: Use generics for type-safe attributes
3. **Metrics integration**: Export log volume/rates as metrics

## Related Documentation

- [HANDLERS.md](HANDLERS.md): Handler comparison and selection guide
- [HANDLER_DEVELOPMENT.md](HANDLER_DEVELOPMENT.md): Implementing custom handlers
- [COMPATIBILITY.md](COMPATIBILITY.md): Version policy and stability guarantees