[![GoDoc](https://pkg.go.dev/badge/github.com/balinomad/go-unilog/handler/logrus?status.svg)](https://pkg.go.dev/github.com/balinomad/go-unilog/handler/logrus?tab=doc)
[![GoMod](https://img.shields.io/github/go-mod/go-version/balinomad/go-unilog)](https://github.com/balinomad/go-unilog)
[![License](https://img.shields.io/github/license/balinomad/go-unilog)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/balinomad/go-unilog/handler/logrus)](https://goreportcard.com/report/github.com/balinomad/go-unilog/handler/logrus)
[![codecov](https://codecov.io/github/balinomad/go-unilog/graph/badge.svg?token=H04BI4TX2C&flag=handler-logrus)](https://codecov.io/github/balinomad/go-unilog/tree/main/handler/logrus)

# Handler: logrus

Adapter for [`logrus`](https://github.com/sirupsen/logrus) - a structured logger with hooks.

## Features

- **Native caller support**: Uses logrus's `SetReportCaller()`
- **Context propagation**: Passes context to `WithContext()`
- **Hook support**: Compatible with logrus hooks ecosystem
- **Format options**: JSON or text output
- **Dynamic level**: Runtime level changes
- **Stack traces**: Automatic for error-level logs
- **Structured fields**: Native field support

## Installation

```bash
go get github.com/balinomad/go-unilog/handler/logrus
go get github.com/sirupsen/logrus
```

**Requirements**: Go 1.24+ (`unilog` requires Go 1.24)

## Quick Start

```go
package main

import (
    "context"
    "os"

    "github.com/balinomad/go-unilog"
    "github.com/balinomad/go-unilog/handler/logrus"
)

func main() {
    // Create logrus handler
    handler, _ := logrus.New(
        logrus.WithOutput(os.Stdout),
        logrus.WithFormat("json"),
        logrus.WithLevel(unilog.InfoLevel),
        logrus.WithCaller(true),
    )

    // Wrap in unilog.Logger
    logger, _ := unilog.NewLogger(handler)

    ctx := context.Background()
    logger.Info(ctx, "server started", "port", 8080)
}
```

**Output** (JSON):
```json
{"level":"info","msg":"server started","port":8080,"time":"2024-01-15T10:30:00Z"}
```

## Configuration Options

### WithLevel(level)

Set minimum log level.
```go
handler, _ := logrus.New(logrus.WithLevel(unilog.DebugLevel))
```

**Available levels**: TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel, FatalLevel, PanicLevel

**Default**: `InfoLevel`

### WithOutput(writer)

Set output destination.
```go
handler, _ := logrus.New(logrus.WithOutput(os.Stderr))

// Write to file
f, _ := os.Create("app.log")
handler, _ := logrus.New(logrus.WithOutput(f))
```

**Default**: `os.Stderr`

### WithFormat(format)

Set output format.
```go
handler, _ := logrus.New(logrus.WithFormat("json")) // JSON output
handler, _ := logrus.New(logrus.WithFormat("text")) // Text output
```

**Valid formats**: `"json"`, `"text"`

**Default**: `"text"`

### WithCaller(enabled)

Enable source location in logs.
```go
handler, _ := logrus.New(logrus.WithCaller(true))
```

**Output includes**: `"func":"main.main","file":"main.go:20"`

**Default**: `false` (disabled)

**Implementation**: Native via `SetReportCaller()`

### WithTrace(enabled)

Enable stack traces for error-level logs.
```go
handler, _ := logrus.New(logrus.WithTrace(true))
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
{"level":"info","msg":"user logged in","time":"2024-01-15T10:30:00Z","user_id":12345,"ip":"192.168.1.1"}
{"level":"error","msg":"database connection failed","time":"2024-01-15T10:30:01Z","host":"db.example.com","error":"connection timeout"}
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
{"level":"info","msg":"query executed","time":"...","database_query":"SELECT * FROM users","database_duration_ms":15}
```

**Note**: Groups are emulated via key prefixing since logrus lacks native grouping.

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
{"level":"info","msg":"request processed","service":"api","version":"v1.2.3","request_id":"abc-123","user_id":456,"duration_ms":42,"time":"..."}
```

### Dynamic Level Changes

```go
// Start at INFO level
handler, _ := logrus.New(logrus.WithLevel(unilog.InfoLevel))
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
// Add trace ID to context
ctx := context.WithValue(context.Background(), "trace_id", "xyz-789")

logger.Info(ctx, "processing request")
// Context is passed to logrus backend
```

### Text Format for Development

```go
handler, _ := logrus.New(
    logrus.WithFormat("text"),
    logrus.WithLevel(unilog.DebugLevel),
    logrus.WithCaller(true),
)

logger, _ := unilog.NewLogger(handler)
logger.Info(ctx, "starting server", "port", 8080)
```

**Output** (text format):
```
time="2024-01-15T10:30:00Z" level=info msg="starting server" func=main.main file=main.go:15 port=8080
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

handler, _ := logrus.New(
    logrus.WithOutput(rotator),
    logrus.WithFormat("json"),
)
```

### Error with Stack Trace

```go
handler, _ := logrus.New(logrus.WithTrace(true))
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
  "time":"...",
  "error":"connection timeout",
  "stack":"goroutine 1 [running]:\n..."
}
```

### Using Logrus Hooks

```go
// Create handler with logrus-specific configuration
handler, _ := logrus.New(logrus.WithOutput(os.Stdout))

// Access underlying logrus.Logger (type assertion required)
if lr, ok := handler.(*logrusHandler); ok {
    // Add custom hook
    lr.logger.AddHook(myCustomHook)
}

logger, _ := unilog.NewLogger(handler)
```

**Note**: Direct access to underlying logger breaks abstraction but enables logrus-specific features.

## Performance

### Allocation Profile

**Typical log call** (0-4 attributes):
- 4-6 allocations: Record struct, fields map, backend formatting

**Performance characteristics**:
- Native caller support (no emulation overhead)
- Context propagation (minimal overhead)
- Synchronous writes (no buffering)

### Benchmark Results

Preliminary measurements (vs direct logrus):
```
BenchmarkLogrusHandler_Info-8   620ns ± 2%
BenchmarkLogrusDirect-8         600ns ± 1%
```

**Overhead**: ~20ns per log call (~3.3% vs direct logrus)

### Optimization Tips

1. **Disable caller if not needed**: Reduces overhead
2. **Disable trace if not needed**: Avoids stack capture
3. **Use JSON format**: Faster than text format
4. **Minimize attributes**: Each field adds allocation
5. **Reuse contexts**: Chain attributes for repeated fields

## Level Mapping

| unilog Level | logrus Level | Notes |
|--------------|--------------|-------|
| Trace | Trace | Native |
| Debug | Debug | Native |
| Info | Info | Native |
| Warn | Warn | Native |
| Error | Error | Native |
| Critical | Error | No native Critical level |
| Fatal | Fatal | Exits process after log |
| Panic | Panic | Panics after log |

**Note**: logrus has no native Critical level. Mapped to Error.

## Supported Interfaces

- ✅ `handler.Handler`: Core interface
- ✅ `handler.Chainer`: WithAttrs, WithGroup
- ✅ `handler.AdvancedHandler`: WithLevel, WithOutput, WithCallerSkip, etc.
- ✅ `handler.MutableConfig`: SetLevel, SetOutput
- ❌ `handler.Syncer`: Not applicable (synchronous writes)

## Known Limitations

1. **No native grouping**: Groups emulated via key prefixing
2. **No native Critical level**: Mapped to Error
3. **No buffering**: Writes synchronously, no `Sync()` needed
4. **Hook compatibility**: Direct hook access breaks abstraction

## Troubleshooting

### Caller Location Not Shown

**Problem**: Logs don't include source file/line

**Solution**: Enable caller reporting:
```go
handler, _ := logrus.New(logrus.WithCaller(true))
```

### Wrong Caller Reported

**Problem**: Source location points to wrapper, not call site

**Cause**: logrus's native caller detection may need adjustment

**Solution**: Adjust caller skip (advanced):
```go
logger, _ := unilog.NewLogger(handler)
advLogger := logger.(unilog.AdvancedLogger)
correctedLogger := advLogger.WithCallerSkip(1) // Skip one extra frame
```

### Grouped Keys Too Long

**Problem**: Nested groups create unwieldy keys like `service_database_connection_pool_size`

**Solution**: Avoid deep nesting, use shorter group names

### Performance Degradation

**Problem**: Logging slower than expected

**Checklist**:
- Disable caller if not needed (`WithCaller(false)`)
- Disable trace if not needed (`WithTrace(false)`)
- Use JSON format (faster than text)
- Minimize number of fields per log
- Profile with `go test -bench . -benchmem`

## Comparison with Direct logrus

| Feature | unilog + logrus | Direct logrus |
|---------|-----------------|---------------|
| API | Unified interface | logrus-specific |
| Swappable | Yes | No |
| Overhead | ~20ns | 0ns |
| Caller | Native | Native |
| Context | Passed through | Passed through |
| Hooks | Via direct access | Native |

**When to use unilog + logrus**: Need unified interface, existing logrus codebase

**When to use logrus directly**: Need hooks, maximum performance, no abstraction

## Migration from Direct logrus

### Before (Direct logrus)

```go
import "github.com/sirupsen/logrus"

logger := logrus.New()
logger.WithFields(logrus.Fields{
    "key": "value",
}).Info("message")
```

### After (unilog + logrus)

```go
import "github.com/balinomad/go-unilog/handler/logrus"

handler, _ := logrus.New(logrus.WithFormat("json"))
logger, _ := unilog.NewLogger(handler)

logger.Info(ctx, "message", "key", "value")
```

**Changes**:
- Add `context.Context` parameter
- Use key-value pairs instead of `WithFields()`
- Configuration via constructor options

## Related Documentation

- [unilog README](../../README.md): Main library documentation
- [Handler Comparison](../../docs/HANDLERS.md): Compare with other handlers
- [Architecture](../../docs/ARCHITECTURE.md): System design details
- [logrus godoc](https://pkg.go.dev/github.com/sirupsen/logrus): Official logrus documentation

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

To report logrus-specific issues, open an issue on [GitHub](https://github.com/balinomad/go-unilog/issues).