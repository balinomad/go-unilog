# Handler Implementations

## Overview

Handlers adapt third-party logging libraries to the `handler.Handler` interface. Each handler provides:

- **Level mapping**: Translates unilog levels to backend-specific levels
- **Attribute conversion**: Converts key-value pairs to backend format
- **Optional features**: Implements additional interfaces as supported

### Overall Handler Implementation Strategy

## Handler Implementation Strategy

The handler implementations follow layered architecture with clear responsibilities:

```
┌─────────────────────────────────────────┐
│     Handler-Specific Configuration      │  ← handler/<name>/<name>.go
├─────────────────────────────────────────┤
│         BaseHandler (Common)            │  ← handler/base.go
├─────────────────────────────────────────┤
│    Backend Logger (zap, slog, etc.)     │  ← Third-party library
└─────────────────────────────────────────┘
```

### Layer Responsibilities

| Layer | Owns | Examples |
|-------|------|----------|
| `BaseHandler` | Level checking, output swapping, format validation, caller skip tracking, optional prefix management | `Enabled()`, `SetLevel()`, `ApplyPrefix()` |
| `Handler` (adapter) | Backend interfacing, attribute conversion, level mapping, strategy selection | `Handle()`, `WithAttrs()` |
| Backend (logger) | Actual log writing, encoding, native features | `zap.Logger.Info()`, `slog.LogAttrs()` |

## Interface Support Matrix

| Handler     | Status | Chainer | AdvancedHandler | Configurator | Syncer | Notes |
|-------------|--------|---------|-----------------|--------------|--------|-------|
| **slog**    | ✅ Stable | ✅ | ✅ | ✅ | ❌ | Standard library, native features |
| **zap**     | ✅ Stable | ✅ | ✅ | ✅ | ✅ | High performance, full feature set |
| **stdlog**  | ✅ Stable | ✅ | ✅ | ✅ | ❌ | Minimal dependencies, simple |
| **zerolog** | 🚧 In development | 🚧 | 🚧 | 🚧 | ❌ | Zero-allocation design |
| **logrus**  | 🚧 In development | 🚧 | 🚧 | 🚧 | ❌ | Structured logging with hooks |
| **log15**   | 🚧 In development | 🚧 | 🚧 | 🚧 | ❌ | Terminal-friendly formatting |

**Legend**: ✅ Implemented | ❌ Not supported | 🚧 In development

## Log Level Mapping

Not every handler has the same log levels. To maintain consistent behavior, we map unilog levels to the nearest semantic equivalent in each backend.

| `unilog` Level | slog      | zap    | stdlog   | zerolog 🚧 | logrus 🚧 | log15 🚧 |
|----------------|-----------|--------|----------|----------|---------|------------|
| **Trace**      | Level(-8) | Debug* | DEBUG*   | Trace    | Trace   | Debug*     |
| **Debug**      | Debug     | Debug  | DEBUG    | Debug    | Debug   | Debug      |
| **Info**       | Info      | Info   | INFO     | Info     | Info    | Info       |
| **Warn**       | Warn      | Warn   | WARN     | Warn     | Warn    | Warn       |
| **Error**      | Error     | Error  | ERROR    | Error    | Error   | Error      |
| **Critical**   | Level(12) | Error* | CRITICAL | Error*   | Error*  | Crit       |
| **Fatal**      | Level(16) | Fatal  | FATAL    | Fatal    | Fatal   | Crit+Exit  |
| **Panic**      | Level(20) | Panic  | PANIC    | Panic    | Panic   | Crit+Panic |

`*` No native equivalent, mapped to nearest semantic level
`🚧` Handler implementation in progress

## Context Handling

### Context Cancellation
All handlers respect context cancellation at the wrapper level. If `ctx.Err() != nil`, logging is skipped before reaching the handler.

## Feature Support Matrix

### Native Backend Features

| Feature | slog | zap | stdlog | zerolog 🚧 | logrus 🚧 | log15 🚧 |
|---------|------|-----|--------|----------|---------|--------|
| **Caller Skip** | ✅ Native | ✅ Native | ❌ Emulated | ❌ Emulated | ❌ Emulated | ❌ Emulated |
| **Grouping** | ✅ WithGroup | ✅ Namespace | ❌ Prefix | ❌ Prefix | ❌ Prefix | ❌ Prefix |
| **Context Propagation** | ✅ Handle(ctx) | ❌ N/A | ❌ N/A | ❌ N/A | ✅ WithContext | ❌ N/A |
| **Dynamic Level** | ✅ LevelVar | ✅ AtomicLevel | ✅ BaseHandler | 🚧 | 🚧 | 🚧 |
| **Dynamic Output** | ✅ BaseHandler | ✅ BaseHandler | ✅ BaseHandler | 🚧 | 🚧 | 🚧 |
| **Buffered Output** | ❌ Synchronous | ✅ Sync() | ❌ Synchronous | ❌ Synchronous | ❌ Synchronous | ❌ Synchronous |

`🚧` Feature support subject to change during development

### Handler Capabilities

| Capability | slog | zap | stdlog | zerolog¹ | logrus¹ | log15¹ |
|------------|------|-----|--------|----------|---------|--------|
| **Format Options** | JSON, Text | JSON, Console | Text | JSON, Console | JSON, Text | JSON, Term |
| **Caller Reporting** | ✅ | ✅ | ✅ | 🚧 | 🚧 | 🚧 |
| **Stack Traces** | ✅ | ✅ | ✅ | 🚧 | 🚧 | 🚧 |
| **Custom Attributes** | ✅ | ✅ | ✅ | 🚧 | 🚧 | 🚧 |
| **Attribute Groups** | ✅ | ✅ | ✅ | 🚧 | 🚧 | 🚧 |

## Performance Characteristics

### Implementation Status

**Current**: Formal benchmarks pending. Handlers will be benchmarked once all implementations are finalized.

**Preliminary observations** (actual benchmarks will replace these):
- slog: stdlib overhead, reliable baseline
- zap: optimized for high throughput
- stdlog: minimal feature overhead

### Allocation Profile (Estimated - per log call)

| Handler | Allocations | Status |
|---------|-------------|--------|
| slog | 3-5 | Measured (stable) |
| zap | 2-4 | Measured (stable) |
| stdlog | 3-5 | Measured (stable) |
| zerolog | 1-3 (target) | In development |
| logrus | 4-6 (estimated) | In development |
| log15 | 3-5 (estimated) | In development |

**Note**: Allocation counts are preliminary and will be replaced with benchmark results.

### Performance Categories

Handler methods fall into three performance categories:

### Hot Path (Category C): Absolute Critical

- `Handle()`, `Enabled()`: Called on every log attempt
- **Target**: < 10ns overhead vs direct backend
- **Strategy**: Cache all flags, use atomics, zero locks

### Warm Path (Category B): Relatively Frequent

- `WithAttrs()`, `WithGroup()`: Called during request setup
- **Target**: < 200ns per call
- **Strategy**: Shallow copy (share mutable state)

### Cold Path (Category A): Rare

- `WithLevel()`, `SetLevel()`, `New()`: Called once per module/reconfiguration
- **Target**: < 2μs per call
- **Strategy**: Deep copy or full reconstruction acceptable

## Caller Skip Behavior

### Overview

Caller skip adjusts which stack frame is reported as the log call site. This is essential for wrapper libraries to report the correct caller location.

### Default Skip Values

Each handler has a base skip value accounting for internal frames:

| Handler | Base Skip | Implementation | Reason |
|---------|-----------|----------------|--------|
| slog | 0 | Native PC capture | slog infers caller automatically |
| zap | 0 | Native AddCallerSkip | zap's AddCallerSkip handles internal |
| stdlog | 0 | PC capture | Uses Record.PC for caller detection |
| zerolog¹ | TBD | TBD | Implementation pending |
| logrus¹ | TBD | TBD | Implementation pending |
| log15¹ | TBD | TBD | Implementation pending |

`¹` Skip values will be determined during implementation

### Caller Detection Strategies

**Native Skip (zap)**: Backend accepts skip parameter directly
```go
// Handler passes Record.Skip to backend
h.backend.WithCallerSkip(r.Skip).Log(...)
```

**Native PC (slog)**: Backend uses program counter from Record
```go
// Handler passes Record.PC to slog.NewRecord
rec := slog.NewRecord(r.Time, level, r.Message, r.PC)
```

**Emulated (stdlog)**: Handler resolves caller from PC
```go
// Handler uses runtime.FuncForPC to get location
if r.PC != 0 {
    frame := runtime.FuncForPC(r.PC)
    file, line := frame.FileLine(r.PC)
    fields.Set("source", fmt.Sprintf("%s:%d", file, line))
}
```

## Context Handling

### Context Cancellation

All handlers respect context cancellation at the unilog wrapper level. If `ctx.Err() != nil`, logging is skipped before reaching the handler.

### Context Propagation

Handlers forward context to backends when supported:

| Handler | Context Support | Implementation |
|---------|-----------------|----------------|
| slog | ✅ Full | Passes context to `Handle(ctx, record)` |
| zap | ❌ None | Zap does not accept context in log calls |
| stdlog | ❌ None | No context support |
| zerolog¹ | TBD | Implementation pending |
| logrus¹ | TBD | Expected: `WithContext()` when ctx non-nil |
| log15¹ | TBD | Implementation pending |

`¹` Context handling will be determined during implementation

**Future**: Context-based trace ID extraction and propagation may be added to unilog wrapper layer.

## Handler Selection Guide

### Use slog When

- ✅ You want standard library only (no external dependencies)
- ✅ Starting a new project
- ✅ Go 1.21+ is acceptable
- ✅ Native grouping and caller support are important
- ✅ Moderate performance is sufficient

**Best for**: New applications, stdlib-focused projects, educational use

### Use zap When

- ✅ High-throughput logging is critical
- ✅ Zero-allocation hot path is needed
- ✅ You need buffered output with explicit sync
- ✅ Full feature set (caller, trace, grouping) required
- ✅ Performance benchmarking shows zap advantage

**Best for**: High-performance services, microservices, production workloads

### Use stdlog When

- ✅ Simplicity over features is priority
- ✅ Minimal dependencies required
- ✅ Standard library only (including unilog dependencies)
- ✅ Basic structured logging sufficient
- ✅ No advanced features needed

**Best for**: Simple applications, legacy integration, minimal deployments

### Use zerolog When 🚧

- 🚧 Zero-allocation is absolute requirement
- 🚧 Performance-critical applications
- 🚧 Memory efficiency is paramount

**Best for**: Ultra-high-performance services, embedded systems (pending completion)

### Use logrus When 🚧

- 🚧 Existing logrus codebase
- 🚧 Hook-based logging required
- 🚧 Popular ecosystem integrations

**Best for**: Migration from logrus, hook-based workflows (pending completion)

### Use log15 When 🚧

- 🚧 Terminal-friendly output required
- 🚧 Human-readable logs priority
- 🚧 Development-focused logging

**Best for**: CLI tools, development logging (pending completion)

`🚧` Recommendations subject to change after implementation

## Handler-Specific Documentation

Detailed configuration and examples for each handler:

- **[slog](../handler/slog/README.md)**: Standard library adapter
- **[zap](../handler/zap/README.md)**: High-performance adapter
- **[stdlog](../handler/stdlog/README.md)**: Minimal dependency adapter
- **zerolog**: Documentation pending implementation
- **logrus**: Documentation pending implementation
- **log15**: Documentation pending implementation

## Creating Custom Handlers

Want to adapt a different logging library? See [HANDLER_DEVELOPMENT.md](HANDLER_DEVELOPMENT.md) for:

- Implementation checklist
- Interface requirements
- Testing guidelines
- Common patterns and pitfalls
- Performance optimization strategies

## Benchmark Methodology (Planned)

Once all handlers are finalized, benchmarks will measure:

1. **Hot path overhead**: Handler.Handle() vs direct backend call
2. **Allocation count**: Per log call with 0-8 attributes
3. **Throughput**: Logs per second under contention
4. **Memory usage**: Heap allocations over 10M log calls

Benchmark suite location: `handler/bench/` (to be created)

Results will be published in this document and included in CI reporting.

## Migration Notes

### From Direct Backend Use

No migration required - handlers are designed to be drop-in replacements with equivalent performance characteristics.

### Between Handlers

Switching handlers requires only changing the constructor:

```go
// Before: zap
handler, _ := zap.New(zap.WithLevel(unilog.InfoLevel))

// After: slog
handler, _ := slog.New(slog.WithLevel(unilog.InfoLevel))

// Application code unchanged
logger, _ := unilog.NewLogger(handler)
```

Configuration options differ between handlers - see handler-specific documentation.

## Version Compatibility

| unilog Version | Go Version | Handler Status |
|----------------|------------|----------------|
| v0.x (current) | 1.24+ | slog, zap, stdlog stable; others in development |
| v1.0 (planned) | 1.24+ | All handlers stable |

See [COMPATIBILITY.md](COMPATIBILITY.md) for version policy details.