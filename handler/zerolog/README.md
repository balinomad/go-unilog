[![GoDoc](https://pkg.go.dev/badge/github.com/balinomad/go-unilog/handler/zerolog?status.svg)](https://pkg.go.dev/github.com/balinomad/go-unilog/handler/zerolog?tab=doc)
[![GoMod](https://img.shields.io/github/go-mod/go-version/balinomad/go-unilog)](https://github.com/balinomad/go-unilog)
[![License](https://img.shields.io/github/license/balinomad/go-unilog)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/balinomad/go-unilog/handler/zerolog)](https://goreportcard.com/report/github.com/balinomad/go-unilog/handler/zerolog)
[![codecov](https://codecov.io/github/balinomad/go-unilog/graph/badge.svg?token=H04BI4TX2C&flag=handler-zerolog)](https://codecov.io/github/balinomad/go-unilog/tree/main/handler/zerolog)

# Handler: zerolog

Adapter for [`zerolog`](https://github.com/rs/zerolog) - a zero-allocation JSON logger.

## Features

- **Zero-allocation design**: Optimized for minimal memory overhead
- **Native caller support**: Uses zerolog's `CallerWithSkipFrameCount()`
- **Native grouping**: Via context fields
- **Stack traces**: Automatic via `Stack()` for error-level logs
- **Dynamic level**: Runtime level changes
- **Format options**: JSON or console output
- **High performance**: Sub-microsecond logging

## Installation

```bash
go get github.com/balinomad/go-unilog/handler/zerolog
go get github.com/rs/zerolog
```

**Requirements**: Go 1.24+ (`unilog` requires Go 1.24)

## Quick Start

```go
package main

import (
    "context"
    "os"

    "github.com/balinomad/go-unilog"
    "github.com/balinomad/go-unilog/handler/zerolog"
)

func main() {
    // Create zerolog handler
    handler, _ := zerolog.New(
        zerolog.WithOutput(os.Stdout),
        zerolog.WithFormat("json"),
        zerolog.WithLevel(unilog.InfoLevel),
        zerolog.WithCaller(true),
    )

    // Wrap in unilog.Logger
    logger, _ := unilog.NewLogger(handler)

    ctx := context.Background()
    logger.Info(ctx, "server started", "port", 8080)
}
```

**Output** (JSON):
```json
{"level":"info","time":"2024-01-15T10:30:00Z","caller":"main.go:20","message":"server started","port":8080}
```

## Configuration Options

### WithLevel(level)

Set minimum log level.
```go
handler, _ := zerolog.New(zerolog.WithLevel(unilog.DebugLevel))
```

**Available levels**: TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel, FatalLevel, PanicLevel

**Default**: `InfoLevel`

### WithOutput(writer)

Set output destination.
```go
handler, _ := zerolog.New(zerolog.WithOutput(os.Stderr))

// Write to file
f, _ := os.Create("app.log")
handler, _ := zerolog.New(zerolog.WithOutput(f))
```

**Default**: `os.Stderr`

### WithFormat(format)

Set output format.
```go
handler, _ := zerolog.New(zerolog.WithFormat("json"))    // JSON lines (default)
handler, _ := zerolog.New(zerolog.WithFormat("console")) // Human-readable
```

**Valid formats**: `"json"`, `"console"`

**Default**: `"json"`

### WithCaller(enabled)

Enable source location in logs.
```go
handler, _ := zerolog.New(zerolog.WithCaller(true))
```

**Output includes**: `"caller":"file.go:42"`

**Default**: `false` (disabled)

**Implementation**: Native via `CallerWithSkipFrameCount()`

**Performance impact**: ~5-10ns per log call when enabled

### WithTrace(enabled)

Enable stack traces for error-level logs.
```go
handler, _ := zerolog.New(zerolog.WithTrace(true))
```

**Adds**: `"stack":"..."` for ERROR and above

**Default**: `false` (disabled)

## Examples

### Basic Logging

```go
ctx := context.Background()
logger.Info(ctx, "user logged in", "user_id", 12345, "ip", "192.168.1.1")
logger.Error(ctx, "database connection failed", "host", "db.example.com", "error", err)
```

**Output** (JSON):
```json
{"level":"info","time":"2024-01-15T10:30:00Z","message":"user logged in","user_id":12345,"ip":"192.168.1.1"}
{"level":"error","time":"2024-01-15T10:30:01Z","message":"database connection failed","host":"db.example.com","error":"connection timeout"}
```

### High-Performance Logging

```go
// Zero-allocation with typed fields
logger.Info(ctx, "request completed",
    "method", "GET",           // string
    "status", 200,             // int
    "duration_ms", int64(42),  // int64
    "bytes_sent", uint64(1024),// uint64
    "success", true)           // bool
```

### Structured Logging with Groups
```go
// Create grouped logger
dbLogger := logger.WithGroup("database")

dbLogger.Info(ctx, "query executed",
    "query", "SELECT * FROM users",
    "duration_ms", 15)
```

**Output** (JSON):
```json
{"level":"info","time":"...","group":"database","message":"query executed","query":"SELECT * FROM users","duration_ms":15}
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

**Output**:
```json
{"level":"info","time":"...","service":"api","version":"v1.2.3","request_id":"abc-123","user_id":456,"message":"request processed","duration_ms":42}
```

### Dynamic Level Changes

```go
// Start at INFO level
handler, _ := zerolog.New(zerolog.WithLevel(unilog.InfoLevel))
logger, _ := unilog.NewLogger(handler)

// Production: info only
logger.Info(ctx, "normal operation")   // Logged
logger.Debug(ctx, "debug details")     // Skipped

// Debug incident: enable DEBUG
logger.SetLevel(unilog.DebugLevel)
logger.Debug(ctx, "now visible")       // Logged
```

### Console Format for Development

```go
handler, _ := zerolog.New(
    zerolog.WithFormat("console"),
    zerolog.WithLevel(unilog.DebugLevel),
    zerolog.WithCaller(true),
)

logger, _ := unilog.NewLogger(handler)
logger.Info(ctx, "starting server", "port", 8080)
```

**Output** (console format):
```
10:30:00 INF starting server caller=main.go:15 port=8080
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

handler, _ := zerolog.New(
    zerolog.WithOutput(rotator),
    zerolog.WithFormat("json"),
)
```

### Error with Stack Trace

```go
handler, _ := zerolog.New(zerolog.WithTrace(true))
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
  "time":"...",
  "message":"operation failed",
  "error":"connection timeout",
  "stack":[{"func":"main.main","line":"42","source":"main.go"}]
}
```

## Performance

### Allocation Profile

**Zero-allocation hot path**: When using typed fields and pre-allocated contexts

**Typical log call** (0-4 attributes):
- 1-3 allocations: Record struct, minimal zerolog overhead

**Optimization**: Use strongly-typed methods for zero-allocation logging

### Benchmark Results

Preliminary measurements (vs direct zerolog):
```
BenchmarkZerologHandler_Info-8  250ns ± 1%
BenchmarkZerologDirect-8        245ns ± 1%
```

**Overhead**: ~5ns per log call (~2% vs direct zerolog)

### Optimization Tips

1. **Use typed fields**: `int`, `string`, `bool` faster than `interface{}`
2. **Disable caller if not needed**: Saves ~5-10ns per call
3. **Disable trace if not needed**: Saves stack capture overhead
4. **Pre-allocate contexts**: Use `With()` for repeated fields
5. **JSON format**: Faster than console format

## Level Mapping

| unilog Level | zerolog Level | Notes |
|--------------|---------------|-------|
| Trace | Trace | Native |
| Debug | Debug | Native |
| Info | Info | Native |
| Warn | Warn | Native |
| Error | Error | Native |
| Critical | Error | No native Critical level |
| Fatal | Fatal | Exits process after log |
| Panic | Panic | Panics after log |

**Note**: zerolog has no native Critical level. Mapped to Error.

## Supported Interfaces

- ✅ `handler.Handler`: Core interface
- ✅ `handler.Chainer`: WithAttrs, WithGroup
- ✅ `handler.AdvancedHandler`: WithLevel, WithOutput, WithCallerSkip, etc.
- ✅ `handler.MutableConfig`: SetLevel, SetOutput
- ❌ `handler.Syncer`: Not applicable (synchronous writes)

## Known Limitations

1. **No native Critical level**: Mapped to Error
2. **No buffering**: Writes synchronously, no `Sync()` needed
3. **Group emulation**: Groups add a "group" field rather than nesting

## Troubleshooting

### Caller Location Not Shown

**Problem**: Logs don't include source file/line

**Solution**: Enable caller reporting:
```go
handler, _ := zerolog.New(zerolog.WithCaller(true))
```

### Wrong Caller Reported

**Problem**: Source location points to wrapper, not call site

**Solution**: Adjust caller skip (advanced):
```go
logger, _ := unilog.NewLogger(handler)
advLogger := logger.(unilog.AdvancedLogger)
correctedLogger := advLogger.WithCallerSkip(1) // Skip one extra frame
```

### Performance Degradation

**Problem**: Logging slower than expected

**Checklist**:
- Disable caller if not needed (`WithCaller(false)`)
- Disable trace if not needed (`WithTrace(false)`)
- Use JSON format (faster than console)
- Ensure typed fields where possible
- Profile with `go test -bench . -benchmem`

### Console Format Not Colored

**Problem**: Console format lacks colors

**Cause**: zerolog only colors output when writing to a TTY

**Solution**: Colors are automatic when output is a terminal

## Comparison with Direct zerolog

| Feature | unilog + zerolog | Direct zerolog |
|---------|------------------|----------------|
| API | Unified interface | zerolog-specific |
| Swappable | Yes | No |
| Overhead | ~5ns | 0ns |
| Caller | Native | Native |
| Grouping | Via context | Via context |
| Zero-alloc | Yes | Yes |

**When to use unilog + zerolog**: Need unified interface, want zero-allocation

**When to use zerolog directly**: Maximum performance, no abstraction needed

## Migration from Direct zerolog

### Before (Direct zerolog)

```go
import "github.com/rs/zerolog"

logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
logger.Info().Str("key", "value").Msg("message")
```

### After (unilog + zerolog)

```go
import "github.com/balinomad/go-unilog/handler/zerolog"

handler, _ := zerolog.New(zerolog.WithOutput(os.Stdout))
logger, _ := unilog.NewLogger(handler)

logger.Info(ctx, "message", "key", "value")
```

**Changes**:
- Add `context.Context` parameter
- Use key-value pairs instead of chained methods
- Otherwise identical behavior

## Related Documentation

- [unilog README](../../README.md): Main library documentation
- [Handler Comparison](../../docs/HANDLERS.md): Compare with other handlers
- [Architecture](../../docs/ARCHITECTURE.md): System design details
- [zerolog godoc](https://pkg.go.dev/github.com/rs/zerolog): Official zerolog documentation

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

To report zerolog-specific issues, open an issue on [GitHub](https://github.com/balinomad/go-unilog/issues).