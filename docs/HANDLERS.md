# Handler Implementations

## Overview

Handlers adapt third-party logging libraries to the `handler.Handler` interface. Each handler provides:

- **Level mapping**: Translates unilog levels to backend-specific levels
- **Attribute conversion**: Converts `[]Attr` to backend format
- **Optional features**: Implements additional interfaces as supported

### Overall Handler Implementation Strategy

The handler implementations should follow layered architecture with clear responsibilities.

```
┌─────────────────────────────────────────┐
│     Handler-Specific Configuration      │  ← Handler's option.go
├─────────────────────────────────────────┤
│         BaseHandler (Common)            │  ← handler/base.go
├─────────────────────────────────────────┤
│    Backend Logger (zap, slog, etc.)     │  ← Third-party library
└─────────────────────────────────────────┘
```

### Responsibilities

| Layer | Owns | Examples |
|-------|------|----------|
| `BaseHandler` | Level checking, output swapping, format validation, caller skip tracking, optional prefix management | `Enabled()`, `SetLevel()`, `ApplyPrefix()` |
| `Handler` (logger adapter) | Backend interfacing, attribute conversion, level mapping, strategy selection | `Handle()`, `Enabled()` |
| Backend (logger) | Actual log writing, encoding, native features | `zap.Logger.Info()`, `slog.LogAttrs()` |

## Interface Support Matrix

| Handler  | Chainer | Configurator | Syncer | Cloner | Notes                              |
|----------|---------|--------------|--------|--------|------------------------------------|
| slog     | ✓       | ✓            | ✗      | ✓      | Standard library, no sync needed   |
| zap      | ✓       | ✓            | ✓      | ✓      | High performance, all features     |
| zerolog  | ✓       | ✓            | ✗      | ✓      | Zero-allocation design             |
| logrus   | ✓       | ✓            | ✗      | ✓      | Structured logging with hooks      |
| log15    | ✓       | ✓            | ✗      | ✓      | Terminal-friendly formatting       |
| stdlog   | ✓       | ✓            | ✗      | ✓      | Stdlib `log` with structured attrs |

## Log Level Mapping

Not every handler has the same log levels. To main tain consistent behavior, we need to map unilog levels to the nearest semantic equivalent in each handler.

| `unilog` Level | Stdlib    | Slog       | Zap     | Zerolog | Logrus | Log15     |
|----------------|-----------|------------|---------|---------|--------|-----------|
| **Trace**      | DEBUG*    | Level(-8)  | Debug*  | Trace   | Trace  | Debug*    |
| **Debug**      | DEBUG     | Debug      | Debug   | Debug   | Debug  | Debug     |
| **Info**       | INFO      | Info       | Info    | Info    | Info   | Info      |
| **Warn**       | WARN      | Warn       | Warn    | Warn    | Warn   | Warn      |
| **Error**      | ERROR     | Error      | Error   | Error   | Error  | Error     |
| **Critical**   | CRITICAL  | Level(12)  | Error*  | Error*  | Error* | Crit      |
| **Fatal**      | FATAL     | Level(16)  | Fatal   | Fatal   | Fatal  | Crit+Exit |
| **Panic**      | PANIC     | Level(20)  | Panic   | Panic   | Panic  | Crit+Panic|

`*`: no native equivalent, mapped to nearest

## Context Handling

### Context Cancellation
All handlers respect context cancellation at the wrapper level. If `ctx.Err() != nil`, logging is skipped before reaching the handler.

### Context Propagation
Handlers forward context to their backends:

| Handler | Context Support | Notes                                    |
|---------|-----------------|------------------------------------------|
| slog    | Full            | Passes context to `LogAttrs()`           |
| zap     | None            | Zap does not accept context in log calls |
| zerolog | None            | Zerolog does not accept context          |
| logrus  | Full            | Uses `WithContext()` when ctx non-nil    |
| log15   | None            | No context support                       |
| stdlog  | None            | No context support                       |

**Future**: Context-based trace ID extraction and propagation may be added to unilog wrapper layer.

## Performance Characteristics

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

### Allocation Profile (per log call)

| Handler | Allocations | Notes                                        |
|---------|-------------|----------------------------------------------|
| slog    | 3-5         | One Record, attrs slice, backend formatting  |
| zap     | 2-4         | Record, fields slice (zero-alloc for fields) |
| zerolog | 1-3         | Minimal allocations in hot path              |
| logrus  | 4-6         | Entry allocation, fields map                 |
| log15   | 3-5         | Context allocation per log                   |
| stdlog  | 3-5         | Buffer allocation for formatting             |

**Optimization**: For high-throughput scenarios, prefer zap or zerolog. For stdlib simplicity, use slog.

### Benchmark Results (ns/op, lower is better)
```
BenchmarkSlog-8      500ns ± 2%
BenchmarkZap-8       280ns ± 1%
BenchmarkZerolog-8   220ns ± 2%
BenchmarkLogrus-8    650ns ± 3%
BenchmarkLog15-8     580ns ± 2%
BenchmarkStdlog-8    520ns ± 2%
```

*(Run `go test -bench=. -benchmem ./handler/...` for detailed results)*

### Comparison to Industry Standards

| Logger | Hot Path | Warm Path | Cold Path | Mutability Model |
|--------|----------|-----------|-----------|------------------|
| **unilog** | ~15ns | ~150ns | ~1.4μs | Hybrid (3-tier) |
| zap | ~10ns | ~100ns | ~1μs | Mostly immutable |
| zerolog | ~8ns | ~80ns | N/A | Full immutability |
| logrus | ~50ns | ~200ns | ~2μs | Mostly mutable |
| slog | ~20ns | ~120ns | ~1μs | Hybrid |

## Caller Skip Behavior

### Overview

Caller skip adjusts which stack frame is reported as the log call site. This is essential for wrapper libraries to report the correct caller location.

### Default Skip Values

Each handler has a base skip value accounting for internal frames:

| Handler | Base Skip | Reason                                   |
|---------|-----------|------------------------------------------|
| slog    | 0         | Slog infers caller automatically         |
| zap     | 0         | Zap's AddCallerSkip handles internal     |
| zerolog | 2         | Accounts for wrapper + handler.Handle()  |
| logrus  | 2         | Accounts for wrapper + handler.Handle()  |
| log15   | 2         | Accounts for wrapper + handler.Handle()  |
| stdlog  | 2         | Accounts for wrapper + handler.Handle()  |

## Creating Custom Handlers

See [`docs/CONTRIBUTING.md`](CONTRIBUTING.md) for implementation guide.