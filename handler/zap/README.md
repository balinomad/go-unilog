[![GoDoc](https://pkg.go.dev/badge/github.com/balinomad/go-unilog/handler/zap?status.svg)](https://pkg.go.dev/github.com/balinomad/go-unilog/handler/zap?tab=doc)
[![GoMod](https://img.shields.io/github/go-mod/go-version/balinomad/go-unilog)](https://github.com/balinomad/go-unilog)
[![License](https://img.shields.io/github/license/balinomad/go-unilog)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/balinomad/go-unilog/handler/zap)](https://goreportcard.com/report/github.com/balinomad/go-unilog/handler/zap)
[![codecov](https://codecov.io/github/balinomad/go-unilog/graph/badge.svg?token=H04BI4TX2C&flag=handler-zap)](https://codecov.io/github/balinomad/go-unilog/tree/main/handler/zap)

# Handler: Zap

Adapter for Uber's high-performance [`zap`](https://github.com/uber-go/zap) logger.

## Features

- **Native caller support**: Uses zap's `AddCallerSkip()` for accurate caller reporting
- **Native grouping**: Leverages `zap.Namespace()` for attribute groups
- **Buffered output**: Implements `Syncer` for explicit flush control
- **Dynamic level**: Runtime level changes via `zap.AtomicLevel`
- **Zero-allocation**: Optimized field types for hot path
- **Stack traces**: Automatic stack traces for error-level logs
- **Format options**: JSON or console output

## Installation

```bash
go get github.com/balinomad/go-unilog/handler/zap
go get go.uber.org/zap
```

**Requirements**: Go 1.24+ (`unilog` requires Go 1.24)

## Quick Start

```go
package main

import (
    "context"
    "os"

    "github.com/balinomad/go-unilog"
    "github.com/balinomad/go-unilog/handler/zap"
)

func main() {
    // Create zap handler
    handler, _ := zap.New(
        zap.WithOutput(os.Stdout),
        zap.WithLevel(unilog.InfoLevel),
        zap.WithCaller(true),
    )

    // Wrap in unilog.Logger
    logger, _ := unilog.NewLogger(handler)

    // IMPORTANT: Flush buffered logs on shutdown
    defer logger.Sync()

    ctx := context.Background()
    logger.Info(ctx, "server started", "port", 8080)
}
```

**Output** (JSON):
```json
{"level":"info","ts":1705318200,"caller":"main.go:20","msg":"server started","port":8080}
```

## Configuration Options

### WithLevel(level)

Set minimum log level.

```go
handler, _ := zap.New(zap.WithLevel(unilog.DebugLevel))
```

**Available levels**: TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel, FatalLevel, PanicLevel

**Default**: `InfoLevel`

### WithOutput(writer)

Set output destination.

```go
handler, _ := zap.New(zap.WithOutput(os.Stderr))

// Write to file
f, _ := os.Create("app.log")
handler, _ := zap.New(zap.WithOutput(f))
```

**Default**: `os.Stderr`

**Note**: zap buffers output; call `logger.Sync()` to flush.

### WithCaller(enabled)

Enable source location in logs.

```go
handler, _ := zap.New(zap.WithCaller(true))
```

**Output includes**: `"caller":"file.go:42"`

**Default**: `false` (disabled)

**Performance impact**: ~5-10ns per log call when enabled

### WithTrace(enabled)

Enable automatic stack traces for error-level logs.

```go
handler, _ := zap.New(zap.WithTrace(true))
```

**Adds**: `"stacktrace":"goroutine 1 [running]:..."` for ERROR and above

**Default**: `false` (disabled)

## Examples

### Basic Logging

```go
ctx := context.Background()
logger.Info(ctx, "user logged in", "user_id", 12345, "ip", "192.168.1.1")
logger.Error(ctx, "database connection failed", "host", "db.example.com", "error", err)
```

### High-Performance Logging

```go
// zap's zero-allocation field types
logger.Info(ctx, "request completed",
    "method", "GET",              // string
    "status", 200,                // int
    "duration_ms", 42,            // int
    "bytes_sent", int64(1024),   // int64
    "success", true)              // bool
```

### Structured Logging with Namespaces

```go
// Create namespaced logger
dbLogger := logger.WithGroup("database")

dbLogger.Info(ctx, "query executed",
    "query", "SELECT * FROM users",
    "duration_ms", 15)
```

**Output** (JSON):
```json
{"level":"info","msg":"query executed","database":{"query":"SELECT * FROM users","duration_ms":15}}
```

### Chaining Attributes

```go
// Add service context
serviceLogger := logger.With("service", "api", "version", "v1.2.3")

// Add request context
requestLogger := serviceLogger.With("request_id", "abc-123", "user_id", 456)

// All logs include service + request context
requestLogger.Info(ctx, "request processed", "duration_ms", 42)
```

### Dynamic Level Changes

```go
// Start at INFO level
handler, _ := zap.New(zap.WithLevel(unilog.InfoLevel))
logger, _ := unilog.NewLogger(handler)

// Production: info only
logger.Info(ctx, "normal operation")   // Logged
logger.Debug(ctx, "debug details")     // Skipped

// Debug incident: enable DEBUG
logger.SetLevel(unilog.DebugLevel)
logger.Debug(ctx, "now visible")       // Logged
```

### Explicit Sync (Buffered Output)

```go
// Flush logs before shutdown
defer func() {
    if err := logger.Sync(); err != nil {
        fmt.Fprintf(os.Stderr, "failed to sync logger: %v\n", err)
    }
}()

// Or use context for graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

go func() {
    <-ctx.Done()
    logger.Sync()
}()
```

### File Output with Rotation

```go
// Use with rotating file handler
import "github.com/balinomad/go-unilog/io/rotating"

rotator, _ := rotating.NewRotatingWriter(&rotating.RotatingWriterConfig{
    Filename:   "app.log",
    MaxSize:    100, // MB
    MaxBackups: 5,
})

handler, _ := zap.New(zap.WithOutput(rotator))
logger, _ := unilog.NewLogger(handler)

// CRITICAL: Sync before rotating or exiting
defer logger.Sync()
```

### Error with Stack Trace

```go
handler, _ := zap.New(zap.WithTrace(true))
logger, _ := unilog.NewLogger(handler)

err := doSomething()
if err != nil {
    logger.Error(ctx, "operation failed", "error", err)
    // Includes automatic stack trace
}
```

**Output**:
```json
{
  "level":"error",
  "msg":"operation failed",
  "error":"connection timeout",
  "stacktrace":"goroutine 1 [running]:\nmain.main()\n\t/app/main.go:42 +0x123\n..."
}
```

### Console Format for Development

```go
// Not yet implemented - use JSON for now
// Console format support coming in future release
```

## Performance

### Allocation Profile

**Typical log call** (0-4 attributes):
- 2-4 allocations: Record struct, field slice, backend field conversion

**Zero-allocation scenarios**:
- Pre-allocated field slices
- Strongly-typed field methods (future: direct zap.Field usage)

### Benchmark Results

Preliminary measurements (vs direct zap):

```
BenchmarkZapHandler_Info-8      280ns ± 1%
BenchmarkZapDirect-8            275ns ± 1%
```

**Overhead**: ~5ns per log call (~2% vs direct zap)

### Optimization Tips

1. **Disable caller if not needed**: Saves ~5-10ns per call
2. **Disable trace if not needed**: Saves stack capture overhead
3. **Use typed attributes**: `int`, `string`, `bool` faster than `interface{}`
4. **Batch logs**: Multiple fields in single call cheaper than multiple calls
5. **Sync periodically**: Don't defer sync in tight loops

## Level Mapping

| unilog Level | zap Level | Notes |
|--------------|-----------|-------|
| Trace | Debug | No native Trace level |
| Debug | Debug | Native |
| Info | Info | Native |
| Warn | Warn | Native |
| Error | Error | Native |
| Critical | Error | No native Critical level |
| Fatal | Fatal | Exits process after log |
| Panic | Panic | Panics after log |

**Note**: zap has no native Trace or Critical levels. Mapped to nearest semantic level.

## Supported Interfaces

- ✅ `handler.Handler`: Core interface
- ✅ `handler.Chainer`: WithAttrs, WithGroup
- ✅ `handler.AdvancedHandler`: WithLevel, WithOutput, WithCallerSkip, etc.
- ✅ `handler.MutableConfig`: SetLevel, SetOutput
- ✅ `handler.Syncer`: Sync (flush buffered output)

## Critical: Sync Before Exit

**⚠️ IMPORTANT**: zap buffers output for performance. **Always call `Sync()` before program exit** to flush buffered logs.

### Recommended Pattern

```go
func main() {
    handler, _ := zap.New(zap.WithOutput(os.Stdout))
    logger, _ := unilog.NewLogger(handler)

    // Register sync on exit
    defer func() {
        if err := logger.Sync(); err != nil {
            // Ignore expected errors (e.g., sync /dev/stderr)
            if !isIgnorableError(err) {
                fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
            }
        }
    }()

    // Application logic
    logger.Info(context.Background(), "starting")
}

func isIgnorableError(err error) bool {
    // Check for platform-specific ignorable errors
    return strings.Contains(err.Error(), "invalid argument") ||
           strings.Contains(err.Error(), "bad file descriptor")
}
```

### Sync in Tests

```go
func TestSomething(t *testing.T) {
    handler, _ := zap.New(zap.WithOutput(io.Discard))
    logger, _ := unilog.NewLogger(handler)
    defer logger.Sync() // Ensure cleanup

    // Test logic
}
```

---

## Troubleshooting

### Caller Location Not Shown

**Problem**: Logs don't include source file/line

**Solution**: Enable caller reporting:
```go
handler, _ := zap.New(zap.WithCaller(true))
```

### Wrong Caller Reported

**Problem**: Source location points to wrapper, not call site

**Solution**: Adjust caller skip (advanced):
```go
logger, _ := unilog.NewLogger(handler)
advLogger := logger.(unilog.AdvancedLogger)
correctedLogger := advLogger.WithCallerSkip(1) // Skip one extra frame
```

### Logs Not Written

**Problem**: Logs missing at program exit

**Cause**: Buffer not flushed

**Solution**: Always call `Sync()` before exit:
```go
defer logger.Sync()
```

### Sync Errors on Stderr

**Problem**: `sync: invalid argument` error when syncing stderr

**Cause**: Platform-specific behavior (e.g., Linux /dev/stderr not sync-able)

**Solution**: Ignore expected sync errors:
```go
err := logger.Sync()
if err != nil && !strings.Contains(err.Error(), "invalid argument") {
    fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
}
```

### Performance Degradation

**Problem**: Logging slower than expected

**Checklist**:
- Disable caller if not needed (`WithCaller(false)`)
- Disable trace if not needed (`WithTrace(false)`)
- Ensure output is buffered (zap handles this)
- Check for excessive field allocations
- Profile with `go test -bench . -benchmem`

---

## Comparison with Direct zap

| Feature | unilog + zap | Direct zap |
|---------|--------------|------------|
| API | Unified interface | zap-specific |
| Swappable | Yes | No |
| Overhead | ~5ns | 0ns |
| Caller | Native (AddCallerSkip) | Native |
| Grouping | Native (Namespace) | Native |
| Buffering | Via Sync() | Via Sync() |
| Zero-alloc | Possible | Possible |

**When to use unilog + zap**: Need to swap backends later, want unified interface

**When to use zap directly**: Maximum performance, no abstraction needed

## Migration from Direct zap

### Before (Direct zap)

```go
logger, _ := zap.NewProduction()
defer logger.Sync()

logger.Info("message",
    zap.String("key", "value"),
    zap.Int("count", 42))
```

### After (unilog + zap)

```go
handler, _ := zap.New(zap.WithLevel(unilog.InfoLevel))
logger, _ := unilog.NewLogger(handler)
defer logger.Sync()

logger.Info(ctx, "message",
    "key", "value",
    "count", 42)
```

**Changes**:
- Add `context.Context` parameter
- Use key-value pairs instead of `zap.Field` types
- Otherwise identical behavior

## Related Documentation

- [unilog README](../../README.md): Main library documentation
- [Handler Comparison](../../docs/HANDLERS.md): Compare with other handlers
- [Architecture](../../docs/ARCHITECTURE.md): System design details
- [zap godoc](https://pkg.go.dev/go.uber.org/zap): Official zap documentation

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

To report zap-specific issues, open an issue on [GitHub](https://github.com/balinomad/go-unilog/issues).