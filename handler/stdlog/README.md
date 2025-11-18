# stdlog Handler

Adapter for Go's standard library [`log`](https://pkg.go.dev/log) package with structured attribute support.

## Features

- **Minimal dependencies**: Uses only stdlib + small unilog helpers
- **Structured logging**: Key-value pairs in standard log format
- **Caller support**: Emulated via PC resolution
- **Stack traces**: Automatic for error-level logs
- **Attribute grouping**: Via key prefixing
- **Dynamic level**: Runtime level changes
- **Text output**: Human-readable format

## Installation

```bash
go get github.com/balinomad/go-unilog/handler/stdlog
```

**Requirements**: Go 1.24+ (`unilog` requires Go 1.24)

## Quick Start

```go
package main

import (
    "context"
    "os"

    "github.com/balinomad/go-unilog"
    "github.com/balinomad/go-unilog/handler/stdlog"
)

func main() {
    // Create stdlog handler
    handler, _ := stdlog.New(
        stdlog.WithOutput(os.Stdout),
        stdlog.WithLevel(unilog.InfoLevel),
        stdlog.WithCaller(true),
    )

    // Wrap in unilog.Logger
    logger, _ := unilog.NewLogger(handler)

    ctx := context.Background()
    logger.Info(ctx, "server started", "port", 8080)
}
```

**Output**:
```
2024/01/15 10:30:00 [INFO] server started port=8080
```

## Configuration Options

### WithLevel(level)

Set minimum log level.

```go
handler, _ := stdlog.New(stdlog.WithLevel(unilog.DebugLevel))
```

**Available levels**: TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel, FatalLevel, PanicLevel

**Default**: `InfoLevel`

### WithOutput(writer)

Set output destination.

```go
handler, _ := stdlog.New(stdlog.WithOutput(os.Stderr))

// Write to file
f, _ := os.Create("app.log")
handler, _ := stdlog.New(stdlog.WithOutput(f))
```

**Default**: `os.Stderr`

### WithSeparator(separator)

Set separator for grouped attribute keys.

```go
handler, _ := stdlog.New(stdlog.WithSeparator("."))
// Grouped keys: "database.query" instead of "database_query"
```

**Default**: `"_"` (underscore)

### WithCaller(enabled)

Enable source location in logs.

```go
handler, _ := stdlog.New(stdlog.WithCaller(true))
```

**Output includes**: `source=file.go:42`

**Default**: `false` (disabled)

**Implementation**: Emulated via program counter resolution

### WithTrace(enabled)

Enable stack traces for error-level logs.

```go
handler, _ := stdlog.New(stdlog.WithTrace(true))
```

**Adds**: `stack=goroutine 1 [running]:...` for ERROR and above

**Default**: `false` (disabled)

### WithFlags(flags)

Set standard log flags (timestamp format, etc.).

```go
import "log"

handler, _ := stdlog.New(
    stdlog.WithFlags(log.LstdFlags | log.Lshortfile),
)
```

**Common flags**:
- `log.LstdFlags`: Date and time (default)
- `log.Lshortfile`: File and line number
- `log.Lmicroseconds`: Microsecond precision
- `log.LUTC`: Use UTC instead of local time

**Default**: `log.LstdFlags` (date + time)

## Examples

### Basic Logging

```go
ctx := context.Background()
logger.Info(ctx, "user logged in", "user_id", 12345, "ip", "192.168.1.1")
logger.Error(ctx, "database connection failed", "host", "db.example.com", "error", err)
```

**Output**:
```
2024/01/15 10:30:00 [INFO] user logged in user_id=12345 ip=192.168.1.1
2024/01/15 10:30:01 [ERROR] database connection failed host=db.example.com error=connection timeout
```

### Structured Logging with Groups

```go
// Create grouped logger
dbLogger := logger.WithGroup("database")

dbLogger.Info(ctx, "query executed",
    "query", "SELECT * FROM users",
    "duration_ms", 15)
```

**Output**:
```
2024/01/15 10:30:00 [INFO] query executed database_query=SELECT * FROM users database_duration_ms=15
```

### Nested Groups

```go
serviceLogger := logger.WithGroup("service")
dbLogger := serviceLogger.WithGroup("database")

dbLogger.Info(ctx, "connected", "host", "localhost")
```

**Output** (with default separator `_`):
```
2024/01/15 10:30:00 [INFO] connected service_database_host=localhost
```

**Output** (with separator `.`):
```
2024/01/15 10:30:00 [INFO] connected service.database.host=localhost
```

### Chaining Attributes

```go
// Add service context
serviceLogger := logger.With("service", "api", "version", "v1.2.3")

// Add request context
requestLogger := serviceLogger.With("request_id", "abc-123")

// All logs include service + request context
requestLogger.Info(ctx, "request processed", "duration_ms", 42)
```

**Output**:
```
2024/01/15 10:30:00 [INFO] request processed service=api version=v1.2.3 request_id=abc-123 duration_ms=42
```

### Dynamic Level Changes

```go
// Start at INFO level
handler, _ := stdlog.New(stdlog.WithLevel(unilog.InfoLevel))
logger, _ := unilog.NewLogger(handler)

// Production: info only
logger.Info(ctx, "normal operation")   // Logged
logger.Debug(ctx, "debug details")     // Skipped

// Debug incident: enable DEBUG
logger.SetLevel(unilog.DebugLevel)
logger.Debug(ctx, "now visible")       // Logged
```

### Caller Information

```go
handler, _ := stdlog.New(
    stdlog.WithCaller(true),
    stdlog.WithFlags(log.LstdFlags | log.Lmicroseconds),
)

logger, _ := unilog.NewLogger(handler)
logger.Info(ctx, "starting server", "port", 8080)
```

**Output**:
```
2024/01/15 10:30:00.123456 [INFO] starting server port=8080 source=main.go:42
```

### Stack Traces on Errors

```go
handler, _ := stdlog.New(stdlog.WithTrace(true))
logger, _ := unilog.NewLogger(handler)

err := doSomething()
if err != nil {
    logger.Error(ctx, "operation failed", "error", err)
}
```

**Output**:
```
2024/01/15 10:30:00 [ERROR] operation failed error=connection timeout stack=goroutine 1 [running]:
main.doSomething(...)
    /app/main.go:42 +0x123
main.main()
    /app/main.go:20 +0x456
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

handler, _ := stdlog.New(stdlog.WithOutput(rotator))
logger, _ := unilog.NewLogger(handler)
```

### Custom Timestamp Format

```go
import "log"

handler, _ := stdlog.New(
    stdlog.WithFlags(log.LUTC | log.Ldate | log.Ltime | log.Lmicroseconds),
)
```

**Output**:
```
2024/01/15 10:30:00.123456 UTC [INFO] message
```

## Performance

### Allocation Profile

**Typical log call** (0-4 attributes):
- 3-5 allocations: Record struct, key-value slice, string building

**Performance characteristics**:
- Caller emulation via PC resolution (~10-20ns overhead)
- Manual key prefixing for groups (no backend grouping)
- Synchronous writes (no buffering)

### Benchmark Results

Preliminary measurements (vs direct log.Print):

```
BenchmarkStdlogHandler_Info-8   520ns ± 2%
BenchmarkLogDirect-8            500ns ± 1%
```

**Overhead**: ~20ns per log call (~4% vs direct log)

### Optimization Tips

1. **Disable caller if not needed**: Saves ~10-20ns per call
2. **Disable trace if not needed**: Avoids stack capture
3. **Minimize attributes**: Each attribute adds allocation
4. **Use file output**: Buffered by OS

## Level Mapping

| unilog Level | stdlog Output | Notes |
|--------------|---------------|-------|
| Trace | [TRACE] | Custom level |
| Debug | [DEBUG] | Custom level |
| Info | [INFO] | Custom level |
| Warn | [WARN] | Custom level |
| Error | [ERROR] | Custom level |
| Critical | [CRITICAL] | Custom level |
| Fatal | [FATAL] | Exits process after log |
| Panic | [PANIC] | Panics after log |

**Note**: All levels are custom-formatted by handler; stdlib `log` has no native levels.

## Supported Interfaces

- ✅ `handler.Handler`: Core interface
- ✅ `handler.Chainer`: WithAttrs, WithGroup
- ✅ `handler.AdvancedHandler`: WithLevel, WithOutput, WithCallerSkip, etc.
- ✅ `handler.MutableConfig`: SetLevel, SetOutput
- ❌ `handler.Syncer`: Not applicable (synchronous writes)

## Known Limitations

1. **No JSON output**: Text format only
2. **No native grouping**: Groups emulated via key prefixing
3. **No buffering**: Writes synchronously, no `Sync()` needed
4. **Caller emulation**: ~10-20ns overhead vs native backend support
5. **No context propagation**: stdlib `log` doesn't accept context

## Troubleshooting

### Caller Location Not Shown

**Problem**: Logs don't include source file/line

**Solution**: Enable caller reporting:
```go
handler, _ := stdlog.New(stdlog.WithCaller(true))
```

### Wrong Caller Reported

**Problem**: Source location points to wrapper, not call site

**Solution**: Adjust caller skip (advanced):
```go
logger, _ := unilog.NewLogger(handler)
advLogger := logger.(unilog.AdvancedLogger)
correctedLogger := advLogger.WithCallerSkip(1) // Skip one extra frame
```

### Grouped Keys Too Long

**Problem**: Nested groups create unwieldy keys like `service_database_connection_pool_size`

**Solution**: Use shorter separator:
```go
handler, _ := stdlog.New(stdlog.WithSeparator("."))
// Result: service.database.connection.pool.size
```

**Alternative**: Avoid deep nesting, use fewer groups

### Performance Degradation

**Problem**: Logging slower than expected

**Checklist**:
- Disable caller if not needed (`WithCaller(false)`)
- Disable trace if not needed (`WithTrace(false)`)
- Reduce number of attributes per log call
- Ensure output is buffered (file, not stdout directly)

## Comparison with Direct log

| Feature | unilog + stdlog | Direct log |
|---------|-----------------|------------|
| API | Unified interface | log-specific |
| Swappable | Yes | No |
| Structured | Yes (key-value) | No |
| Levels | Yes | No |
| Caller | Emulated | Via flags |
| Groups | Prefixing | N/A |
| Overhead | ~20ns | 0ns |

**When to use unilog + stdlog**: Need structured logging, want unified interface, minimal dependencies

**When to use log directly**: Simplest possible logging, no structure needed

## Migration from stdlib log

### Before (Direct log)

```go
import "log"

log.Printf("user %d logged in from %s", userID, ip)
```

### After (unilog + stdlog)

```go
import "github.com/balinomad/go-unilog/handler/stdlog"

handler, _ := stdlog.New()
logger, _ := unilog.NewLogger(handler)

logger.Info(ctx, "user logged in", "user_id", userID, "ip", ip)
```

**Benefits**:
- Structured output (machine-readable)
- Log levels (filter by severity)
- Consistent interface (swap backends later)

## Related Documentation

- [unilog README](../../README.md): Main library documentation
- [Handler Comparison](../../docs/HANDLERS.md): Compare with other handlers
- [Architecture](../../docs/ARCHITECTURE.md): System design details
- [log godoc](https://pkg.go.dev/log): Official log documentation

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

To report stdlog-specific issues, open an issue on [GitHub](https://github.com/balinomad/go-unilog/issues).