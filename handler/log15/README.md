# log15 Handler

Adapter for [`log15`](https://github.com/inconshreveable/log15) logger with terminal-friendly formatting.

## Features

- **Terminal-friendly output**: Colored, human-readable logs for development
- **Format options**: Terminal, JSON, or logfmt output
- **Attribute chaining**: Via field accumulation
- **Group emulation**: Via key prefixing
- **Dynamic level**: Runtime level changes
- **Caller reporting**: Emulated via PC resolution
- **Stack traces**: Automatic for error-level logs

## Installation

```bash
go get github.com/balinomad/go-unilog/handler/log15
go get github.com/inconshreveable/log15/v3
```

**Requirements**: Go 1.24+ (`unilog` requires Go 1.24)

## Quick Start

```go
package main

import (
    "context"
    "os"

    "github.com/balinomad/go-unilog"
    "github.com/balinomad/go-unilog/handler/log15"
)

func main() {
    // Create log15 handler
    handler, _ := log15.New(
        log15.WithOutput(os.Stdout),
        log15.WithFormat("terminal"),
        log15.WithLevel(unilog.InfoLevel),
        log15.WithCaller(true),
    )

    // Wrap in unilog.Logger
    logger, _ := unilog.NewLogger(handler)

    ctx := context.Background()
    logger.Info(ctx, "server started", "port", 8080)
}
```

**Output** (terminal format, colored):
```
INFO[01-15|10:30:00] server started                           port=8080 source=main.go:20
```

## Configuration Options

### WithLevel(level)

Set minimum log level.
```go
handler, _ := log15.New(log15.WithLevel(unilog.DebugLevel))
```

**Available levels**: TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel, FatalLevel, PanicLevel

**Default**: `InfoLevel`

### WithOutput(writer)

Set output destination.
```go
handler, _ := log15.New(log15.WithOutput(os.Stderr))

// Write to file
f, _ := os.Create("app.log")
handler, _ := log15.New(log15.WithOutput(f))
```

**Default**: `os.Stderr`

### WithFormat(format)

Set output format.
```go
handler, _ := log15.New(log15.WithFormat("terminal")) // Colored, human-readable
handler, _ := log15.New(log15.WithFormat("json"))     // JSON lines
handler, _ := log15.New(log15.WithFormat("logfmt"))   // Key=value format
```

**Valid formats**: `"terminal"`, `"json"`, `"logfmt"`

**Default**: `"terminal"`

### WithCaller(enabled)

Enable source location in logs.
```go
handler, _ := log15.New(log15.WithCaller(true))
```

**Output includes**: `source=file.go:42`

**Default**: `false` (disabled)

**Implementation**: Emulated via PC resolution

### WithTrace(enabled)

Enable stack traces for error-level logs.
```go
handler, _ := log15.New(log15.WithTrace(true))
```

**Adds**: `stack=goroutine 1 [running]:...` for ERROR and above

**Default**: `false` (disabled)

## Examples

### Basic Logging
```go
ctx := context.Background()
logger.Info(ctx, "user logged in", "user_id", 12345, "ip", "192.168.1.1")
logger.Error(ctx, "database connection failed", "host", "db.example.com", "error", err)
```

**Output** (terminal format):
```
INFO[01-15|10:30:00] user logged in                           user_id=12345 ip=192.168.1.1
ERROR[01-15|10:30:01] database connection failed               host=db.example.com error="connection timeout"
```

### Structured Logging with Groups
```go
// Create grouped logger
dbLogger := logger.WithGroup("database")

dbLogger.Info(ctx, "query executed",
    "query", "SELECT * FROM users",
    "duration_ms", 15)
```

**Output** (terminal format):
```
INFO[01-15|10:30:00] query executed                           database_query="SELECT * FROM users" database_duration_ms=15
```

**Note**: Groups are emulated via key prefixing since log15 lacks native grouping.

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
```
INFO[01-15|10:30:00] request processed                        service=api version=v1.2.3 request_id=abc-123 user_id=456 duration_ms=42
```

### Dynamic Level Changes

```go
// Start at INFO level
handler, _ := log15.New(log15.WithLevel(unilog.InfoLevel))
logger, _ := unilog.NewLogger(handler)

// Production: info only
logger.Info(ctx, "normal operation")   // Logged
logger.Debug(ctx, "debug details")     // Skipped

// Debug incident: enable DEBUG
logger.SetLevel(unilog.DebugLevel)
logger.Debug(ctx, "now visible")       // Logged
```

### JSON Format for Production
```go
handler, _ := log15.New(
    log15.WithFormat("json"),
    log15.WithLevel(unilog.InfoLevel),
    log15.WithCaller(true),
)

logger, _ := unilog.NewLogger(handler)
logger.Info(ctx, "starting server", "port", 8080)
```

**Output** (JSON):
```json
{"lvl":"info","t":"2024-01-15T10:30:00Z","msg":"starting server","port":8080,"source":"main.go:15"}
```

### Logfmt Format

```go
handler, _ := log15.New(log15.WithFormat("logfmt"))
logger, _ := unilog.NewLogger(handler)
logger.Info(ctx, "processing request", "method", "GET", "path", "/api/users")
```

**Output** (logfmt):
```
lvl=info t=2024-01-15T10:30:00Z msg="processing request" method=GET path=/api/users
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

handler, _ := log15.New(
    log15.WithOutput(rotator),
    log15.WithFormat("json"),
)
```

### Error with Stack Trace

```go
handler, _ := log15.New(log15.WithTrace(true))
logger, _ := unilog.NewLogger(handler)

err := doSomething()
if err != nil {
    logger.Error(ctx, "operation failed", "error", err)
    // Includes automatic stack trace
}
```

## Performance

### Allocation Profile

**Typical log call** (0-4 attributes):
- 3-5 allocations: Record struct, fields slice, backend formatting

**Performance characteristics**:
- Caller emulation via PC resolution (~10-20ns overhead)
- Manual key prefixing for groups (no backend grouping)
- Synchronous writes (no buffering)

### Benchmark Results

Preliminary measurements (vs direct log15):
```
BenchmarkLog15Handler_Info-8    580ns ± 2%
BenchmarkLog15Direct-8          560ns ± 1%
```

**Overhead**: ~20ns per log call (~3.5% vs direct log15)

### Optimization Tips

1. **Disable caller if not needed**: Saves ~10-20ns per call
2. **Disable trace if not needed**: Avoids stack capture
3. **Use JSON format in production**: Faster parsing than terminal
4. **Minimize attributes**: Each attribute adds allocation

## Level Mapping

| unilog Level | log15 Level | Notes |
|--------------|-------------|-------|
| Trace | Debug | No native Trace level |
| Debug | Debug | Native |
| Info | Info | Native |
| Warn | Warn | Native |
| Error | Error | Native |
| Critical | Crit | Native |
| Fatal | Crit | Mapped to Crit, exits after log |
| Panic | Crit | Mapped to Crit, panics after log |

**Note**: log15 has no native Trace level. Mapped to Debug.

## Supported Interfaces

- ✅ `handler.Handler`: Core interface
- ✅ `handler.Chainer`: WithAttrs, WithGroup
- ✅ `handler.AdvancedHandler`: WithLevel, WithOutput, WithCallerSkip, etc.
- ✅ `handler.MutableConfig`: SetLevel, SetOutput
- ❌ `handler.Syncer`: Not applicable (synchronous writes)

## Known Limitations

1. **No native grouping**: Groups emulated via key prefixing
2. **No native Trace level**: Mapped to Debug
3. **No buffering**: Writes synchronously, no `Sync()` needed
4. **Caller emulation**: ~10-20ns overhead vs native backend support

## Troubleshooting

### Caller Location Not Shown

**Problem**: Logs don't include source file/line

**Solution**: Enable caller reporting:
```go
handler, _ := log15.New(log15.WithCaller(true))
```

### Wrong Caller Reported

**Problem**: Source location points to wrapper, not call site

**Solution**: Adjust caller skip (advanced):
```go
logger, _ := unilog.NewLogger(handler)
advLogger := logger.(unilog.AdvancedLogger)
correctedLogger := advLogger.WithCallerSkip(1) // Skip one extra frame
```

### Colors Not Showing

**Problem**: Terminal format not showing colors

**Cause**: Output not going to a terminal (e.g., redirected to file)

**Solution**: Use JSON or logfmt format for non-terminal output:
```go
handler, _ := log15.New(log15.WithFormat("json"))
```

### Grouped Keys Too Long

**Problem**: Nested groups create unwieldy keys like `service_database_connection_pool_size`

**Solution**: Use shorter separator or avoid deep nesting:
```go
// Not directly configurable in log15 - avoid deep nesting
```

## Comparison with Direct log15

| Feature | unilog + log15 | Direct log15 |
|---------|----------------|--------------|
| API | Unified interface | log15-specific |
| Swappable | Yes | No |
| Overhead | ~20ns | 0ns |
| Caller | Emulated | No native support |
| Grouping | Prefixing | No native support |
| Terminal format | Same | Same |

**When to use unilog + log15**: Need unified interface, want terminal-friendly dev logs

**When to use log15 directly**: Maximum performance, no abstraction needed

## Migration from Direct log15

### Before (Direct log15)

```go
import "github.com/inconshreveable/log15/v3"

logger := log15.New()
logger.Info("message", "key", "value")
```

### After (unilog + log15)

```go
import "github.com/balinomad/go-unilog/handler/log15"

handler, _ := log15.New(log15.WithFormat("terminal"))
logger, _ := unilog.NewLogger(handler)

logger.Info(ctx, "message", "key", "value")
```

**Changes**:
- Add `context.Context` parameter
- Configuration via constructor options
- Otherwise identical behavior

## Related Documentation

- [unilog README](../../README.md): Main library documentation
- [Handler Comparison](../../docs/HANDLERS.md): Compare with other handlers
- [Architecture](../../docs/ARCHITECTURE.md): System design details
- [log15 godoc](https://pkg.go.dev/github.com/inconshreveable/log15/v3): Official log15 documentation

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

To report log15-specific issues, open an issue on [GitHub](https://github.com/balinomad/go-unilog/issues).