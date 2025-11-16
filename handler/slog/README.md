# slog Handler

Adapter for Go's standard library [`log/slog`](https://pkg.go.dev/log/slog) logger.

## Features

- **Native caller support**: Uses slog's PC-based caller detection
- **Native grouping**: Leverages `slog.Handler.WithGroup()`
- **Context propagation**: Passes context to `slog.Handler.Handle()`
- **Dynamic level**: Runtime level changes via `slog.LevelVar`
- **Format options**: JSON or text output
- **Custom attributes**: Transform attributes via `ReplaceAttr`
- **Zero external dependencies**: Stdlib only

## Installation

```bash
go get github.com/balinomad/go-unilog/handler/slog
```

**Requirements**: Go 1.24+ (`unilog` requires Go 1.24)


## Quick Start

```go
package main

import (
    "context"
    "os"

    "github.com/balinomad/go-unilog"
    "github.com/balinomad/go-unilog/handler/slog"
)

func main() {
    // Create slog handler
    handler, _ := slog.New(
        slog.WithOutput(os.Stdout),
        slog.WithFormat("json"),
        slog.WithLevel(unilog.InfoLevel),
        slog.WithCaller(true),
    )

    // Wrap in unilog.Logger
    logger, _ := unilog.NewLogger(handler)

    ctx := context.Background()
    logger.Info(ctx, "server started", "port", 8080)
}
```

**Output** (JSON):
```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","source":"main.go:20","msg":"server started","port":8080}
```

## Configuration Options

### WithLevel(level)

Set minimum log level.

```go
handler, _ := slog.New(slog.WithLevel(unilog.DebugLevel))
```

**Available levels**: TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel, FatalLevel, PanicLevel

**Default**: `InfoLevel`

### WithOutput(writer)

Set output destination.

```go
handler, _ := slog.New(slog.WithOutput(os.Stderr))

// Write to file
f, _ := os.Create("app.log")
handler, _ := slog.New(slog.WithOutput(f))
```

**Default**: `os.Stderr`

### WithFormat(format)

Set output format.

```go
handler, _ := slog.New(slog.WithFormat("json"))  // JSON output
handler, _ := slog.New(slog.WithFormat("text"))  // Human-readable text
```

**Valid formats**: `"json"`, `"text"`

**Default**: `"json"`

### WithCaller(enabled)

Enable source location in logs.

```go
handler, _ := slog.New(slog.WithCaller(true))
```

**Output includes**: `"source":"file.go:42"`

**Default**: `false` (disabled)

**Performance impact**: ~10-20ns per log call when enabled

### WithTrace(enabled)

Enable stack traces for error-level logs.

```go
handler, _ := slog.New(slog.WithTrace(true))
```

**Adds**: `"stack":"goroutine 1 [running]:..."` for ERROR and above

**Default**: `false` (disabled)

### WithReplaceAttr(fn)

Transform attributes before output (slog-specific).

```go
handler, _ := slog.New(
    slog.WithReplaceAttr(func(groups []string, a slog.Attr) slog.Attr {
        // Redact sensitive fields
        if a.Key == "password" {
            return slog.String("password", "[REDACTED]")
        }
        return a
    }),
)
```

**Use cases**:
- Redact sensitive data (passwords, tokens)
- Format timestamps
- Rename keys
- Filter attributes

## Examples

### Basic Logging

```go
ctx := context.Background()
logger.Info(ctx, "user logged in", "user_id", 12345, "ip", "192.168.1.1")
logger.Error(ctx, "database connection failed", "host", "db.example.com", "error", err)
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
{"time":"...","level":"INFO","msg":"query executed","database":{"query":"SELECT * FROM users","duration_ms":15}}
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
{"time":"...","level":"INFO","msg":"request processed","service":"api","version":"v1.2.3","request_id":"abc-123","user_id":456,"duration_ms":42}
```

### Dynamic Level Changes

```go
// Start at INFO level
handler, _ := slog.New(slog.WithLevel(unilog.InfoLevel))
logger, _ := unilog.NewLogger(handler)

// Production: info only
logger.Info(ctx, "normal operation")   // Logged
logger.Debug(ctx, "debug details")     // Skipped

// Debug incident: enable DEBUG
logger.SetLevel(unilog.DebugLevel)
logger.Debug(ctx, "now visible")       // Logged
```

### Context Propagation

```go
// slog passes context to backend
ctx := context.WithValue(context.Background(), "trace_id", "xyz-789")

logger.Info(ctx, "processing request")

// Custom slog handler can extract trace_id from context
```

### Text Format for Development

```go
handler, _ := slog.New(
    slog.WithFormat("text"),
    slog.WithLevel(unilog.DebugLevel),
    slog.WithCaller(true),
)

logger, _ := unilog.NewLogger(handler)
logger.Info(ctx, "starting server", "port", 8080)
```

**Output** (text):
```
time=2024-01-15T10:30:00.000Z level=INFO source=main.go:15 msg="starting server" port=8080
```

### Attribute Transformation

```go
handler, _ := slog.New(
    slog.WithReplaceAttr(func(groups []string, a slog.Attr) slog.Attr {
        // Uppercase level names
        if a.Key == slog.LevelKey {
            level := a.Value.Any().(slog.Level)
            return slog.String(slog.LevelKey, level.String())
        }

        // Format errors
        if a.Key == "error" {
            if err, ok := a.Value.Any().(error); ok {
                return slog.String("error", fmt.Sprintf("%+v", err))
            }
        }

        return a
    }),
)
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

handler, _ := slog.New(
    slog.WithOutput(rotator),
    slog.WithFormat("json"),
)
```

## Performance

### Allocation Profile

**Typical log call** (0-4 attributes):
- 2-3 allocations: Record struct, attribute slice, backend formatting

**Optimizations**:
- Native caller support (PC-based, no stack walk)
- Native grouping (no manual key prefixing)
- Context passed directly to backend

### Benchmark Results

Preliminary measurements (vs direct slog):

```
BenchmarkSlogHandler_Info-8     500ns ± 2%
BenchmarkSlogDirect-8           495ns ± 1%
```

**Overhead**: ~5ns per log call (~1% vs direct slog)

## Level Mapping

| unilog Level | slog Level | Notes |
|--------------|------------|-------|
| Trace | Level(-8) | Custom level below Debug |
| Debug | Debug | Native |
| Info | Info | Native |
| Warn | Warn | Native |
| Error | Error | Native |
| Critical | Level(12) | Custom level above Error |
| Fatal | Level(16) | Custom level, exits after log |
| Panic | Level(20) | Custom level, panics after log |

**Note**: slog has no native Trace/Critical/Fatal/Panic levels. Custom levels are used.

## Supported Interfaces

- ✅ `handler.Handler`: Core interface
- ✅ `handler.Chainer`: WithAttrs, WithGroup
- ✅ `handler.AdvancedHandler`: WithLevel, WithOutput, WithCallerSkip, etc.
- ✅ `handler.Configurator`: SetLevel, SetOutput
- ❌ `handler.Syncer`: Not applicable (slog writes synchronously)

## Known Limitations

1. **No buffering**: slog writes synchronously, no `Sync()` needed
2. **Custom levels**: Trace/Critical/Fatal/Panic are not native slog levels
3. **Go version**: Requires Go 1.21+ (slog introduced in 1.21)

## Troubleshooting

### Caller Location Not Shown

**Problem**: Logs don't include source file/line

**Solution**: Enable caller reporting:
```go
handler, _ := slog.New(slog.WithCaller(true))
```

### Wrong Caller Reported

**Problem**: Source location points to wrapper, not call site

**Solution**: Adjust caller skip (advanced):
```go
logger, _ := unilog.NewLogger(handler)
advLogger := logger.(unilog.AdvancedLogger)
correctedLogger := advLogger.WithCallerSkip(1) // Skip one extra frame
```

### Custom Levels Not Recognized

**Problem**: Trace/Critical logs appear with numeric levels

**Rationale**: These are custom levels, not native to slog. Behavior is expected.

**Workaround**: Use ReplaceAttr to format level names:
```go
slog.WithReplaceAttr(func(groups []string, a slog.Attr) slog.Attr {
    if a.Key == slog.LevelKey {
        level := a.Value.Any().(slog.Level)
        switch level {
        case slog.Level(-8):
            return slog.String(slog.LevelKey, "TRACE")
        case slog.Level(12):
            return slog.String(slog.LevelKey, "CRITICAL")
        // ... etc
        }
    }
    return a
})
```

### Performance Degradation

**Problem**: Logging slower than expected

**Checklist**:
- Disable caller if not needed (`WithCaller(false)`)
- Disable trace if not needed (`WithTrace(false)`)
- Use JSON format (faster than text)
- Ensure output is buffered (file, not os.Stdout directly)

## Comparison with Direct slog

| Feature | unilog + slog | Direct slog |
|---------|---------------|-------------|
| API | Unified interface | slog-specific |
| Swappable | Yes | No |
| Overhead | ~5ns | 0ns |
| Context | Passed through | Passed through |
| Caller | PC-based | PC-based |
| Grouping | Native | Native |

**When to use unilog + slog**: Need to swap backends later, want unified interface

**When to use slog directly**: No need for abstraction, stdlib-only requirement

## Related Documentation

- [unilog README](../../README.md): Main library documentation
- [Handler Comparison](../../docs/HANDLERS.md): Compare with other handlers
- [Architecture](../../docs/ARCHITECTURE.md): System design details
- [slog godoc](https://pkg.go.dev/log/slog): Official slog documentation

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

To report slog-specific issues, open an issue on [GitHub](https://github.com/balinomad/go-unilog/issues).