[![GoDoc](https://pkg.go.dev/badge/github.com/balinomad/go-unilog?status.svg)](https://pkg.go.dev/github.com/balinomad/go-unilog?tab=doc)
[![GoMod](https://img.shields.io/github/go-mod/go-version/balinomad/go-unilog)](https://github.com/balinomad/go-unilog)
[![Size](https://img.shields.io/github/languages/code-size/balinomad/go-unilog)](https://github.com/balinomad/go-unilog)
[![License](https://img.shields.io/github/license/balinomad/go-unilog)](./LICENSE)
[![Go](https://github.com/balinomad/go-unilog/actions/workflows/go.yml/badge.svg)](https://github.com/balinomad/go-unilog/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/balinomad/go-unilog)](https://goreportcard.com/report/github.com/balinomad/go-unilog)
[![codecov](https://codecov.io/github/balinomad/go-unilog/graph/badge.svg?token=L1K68IIN51)](https://codecov.io/github/balinomad/go-unilog)

# unilog

*A unified logging interface for Go applications.*

Stop rewriting logging code when switching backends. Write once against `unilog.Logger`, swap implementations without touching application logic.

## Why unilog?

**Problem**: Direct coupling to logging libraries creates vendor lock-in. Switching from `zerolog` to `slog` requires refactoring every log call site.

**Solution**: A standardized interface with pluggable adapters. Application code stays unchanged; swap the underlying logger in one place.

Perfect for:

- **Libraries**: Let consumers integrate your logs into their existing infrastructure
- **Large Applications**: Enforce consistent logging across teams with different library preferences
- **Evolving Projects**: Start simple, upgrade to high-performance loggers without refactoring

## Quick Start

### Installation

```bash
go get github.com/balinomad/go-unilog@latest
```

### Basic Usage

```go
package main

import (
	"context"
    "os"

	"github.com/balinomad/go-unilog"
	"github.com/balinomad/go-unilog/handler/slog"
)

func main() {
	ctx := context.Background()

    // Create handler (slog in this example)
    handler, _ := slog.New(
        slog.WithOutput(os.Stderr),
        slog.WithFormat("json"),
        slog.WithLevel(unilog.InfoLevel),
    )

    // Wrap in unilog.Logger
    logger, _ := unilog.NewLogger(handler)

    // Log structured data
    logger.Info(ctx, "server started",
        "port", 8080,
        "env", "production")

    logger.Error(ctx, "database connection failed",
        "host", "db.example.com",
        "retry_count", 3)
}
```

## Choosing a Handler

| Handler | Best For | Performance | Features |
|---------|----------|-------------|----------|
| **[slog](handler/slog/)** | Standard library users, new projects | Good | Native caller, groups, context |
| **[zap](handler/zap/)** | High-throughput services | Excellent | Zero-alloc, buffered, full feature set |
| **[stdlog](handler/stdlog/)** | Simple applications, stdlib-only | Moderate | Minimal dependencies |
| **[zerolog](handler/zerolog/)** | Ultra-high performance, zero-alloc | Excellent | Zero-alloc, native caller, native groups |
| **[logrus](handler/logrus/)** | Existing logrus codebases, hooks | Good | Native caller, context, hooks support |
| **[log15](handler/log15/)** | Terminal-friendly development | Good | Colored output, multiple formats |

See [Handler Comparison Matrix](docs/HANDLERS.md) for detailed feature analysis.

## Core Concepts

### Structured Logging

Always log key-value pairs for machine-readable output:

```go
// Good: structured
logger.Info(ctx, "user registered", "user_id", 12345, "email", "user@example.com")

// Avoid: unstructured string formatting
logger.Info(ctx, fmt.Sprintf("User %d registered: %s", 12345, "user@example.com"))
```

### Context Propagation

Pass `context.Context` for request-scoped logging and cancellation awareness:

```go
func HandleRequest(ctx context.Context, logger unilog.Logger) {
    // Context canceled? Logging is skipped automatically
    logger.Info(ctx, "processing request", "request_id", ctx.Value("request_id"))
}
```

### Log Levels

| Level | Use Case | Examples |
|-------|----------|----------|
| `Trace` | Fine-grained debugging | Function entry/exit, loop iterations |
| `Debug` | Development diagnostics | SQL queries, cache hits |
| `Info` | Normal operations | Server started, request completed |
| `Warn` | Recoverable issues | Deprecated API used, retry attempts |
| `Error` | Failure requiring attention | Database timeout, invalid input |
| `Critical` | Severe degradation | Service unavailable, data corruption |
| `Fatal` | Unrecoverable, exits process | Configuration missing, startup failure |

## Advanced Usage

### Chaining Attributes

Avoid repeating common fields:

```go
// Add service-level context
serviceLogger := logger.With("service", "api", "version", "v1.2.3")

// Add request-level context
requestLogger := serviceLogger.With("request_id", "abc-123", "user_id", 456)

// All subsequent logs include service + request context
requestLogger.Info(ctx, "request processed", "duration_ms", 42)
// Output: {"level":"INFO","msg":"request processed","service":"api","version":"v1.2.3","request_id":"abc-123","user_id":456,"duration_ms":42}
```

### Grouping Attributes

Organize related fields hierarchically:

```go
dbLogger := logger.WithGroup("database")
dbLogger.Info(ctx, "query executed",
    "query", "SELECT * FROM users",
    "duration_ms", 15)
// Output: {"level":"INFO","msg":"query executed","database":{"query":"SELECT * FROM users","duration_ms":15}}
```

### Dynamic Configuration

Change log level at runtime (handler must support `MutableConfig` interface):

```go
// Production: INFO level
logger.SetLevel(unilog.InfoLevel)

// Debug incident: enable DEBUG temporarily
logger.SetLevel(unilog.DebugLevel)
```

### Default Logger

Use package-level functions for simple cases:

```go
func init() {
    handler, _ := slog.New(slog.WithLevel(unilog.DebugLevel))
    logger, _ := unilog.NewLogger(handler)
    unilog.SetDefault(logger)
}

func main() {
    ctx := context.Background()
    unilog.Info(ctx, "using default logger")
}
```

## Handler-Specific Examples

### slog (Standard Library)

```go
import "github.com/balinomad/go-unilog/handler/slog"

handler, _ := slog.New(
    slog.WithOutput(os.Stdout),
    slog.WithFormat("text"),        // or "json"
    slog.WithLevel(unilog.DebugLevel),
    slog.WithCaller(true),          // Include source location
)
```

### zap (High Performance)

```go
import "github.com/balinomad/go-unilog/handler/zap"

handler, _ := zap.New(
    zap.WithOutput(os.Stdout),
    zap.WithLevel(unilog.InfoLevel),
    zap.WithCaller(true),
    zap.WithTrace(true),            // Stack traces on errors
)

// Remember to sync buffered output
defer logger.Sync()
```

### stdlog (Minimal Dependencies)

```go
import "github.com/balinomad/go-unilog/handler/stdlog"

handler, _ := stdlog.New(
    stdlog.WithOutput(os.Stderr),
    stdlog.WithLevel(unilog.WarnLevel),
    stdlog.WithFlags(log.LstdFlags | log.Lshortfile),
)
```

See handler-specific README files for detailed configuration options.

## API Reference

### Core Types

- **`Logger`**: Main logging interface (Info, Error, With, WithGroup, etc.)
- **`AdvancedLogger`**: Extends Logger with immutable configuration methods
- **`MutableLogger`**: Runtime reconfiguration (SetLevel, SetOutput)
- **`LogLevel`**: Severity constants (TraceLevel, DebugLevel, InfoLevel, etc.)

### Package Functions

```go
// Default logger management
unilog.SetDefault(logger)
unilog.Default() Logger

// Context integration
unilog.WithLogger(ctx, logger) context.Context
unilog.LoggerFromContext(ctx) (Logger, bool)
unilog.LoggerFromContextOrDefault(ctx) Logger

// Package-level logging (uses default logger)
unilog.Info(ctx, msg, keyValues...)
unilog.Error(ctx, msg, keyValues...)
// ... etc for all levels
```

Full API documentation: [pkg.go.dev/github.com/balinomad/go-unilog](https://pkg.go.dev/github.com/balinomad/go-unilog)

## Testing

unilog provides no built-in test helpers. For testing:

1. **Mock the interface**: Create a `MockLogger` implementing `unilog.Logger`
2. **Use in-memory handler**: Configure handler with `bytes.Buffer` as output
3. **Test handler directly**: Verify `handler.Handle()` behavior

Example mock:

```go
type MockLogger struct {
    Calls []LogCall
}

func (m *MockLogger) Info(ctx context.Context, msg string, keyValues ...any) {
    m.Calls = append(m.Calls, LogCall{Level: InfoLevel, Msg: msg, KVs: keyValues})
}
// ... implement other methods
```

## Performance

unilog adds minimal overhead to underlying loggers. Preliminary observations (formal benchmarks pending):

- **slog**: ~5-10ns per log call
- **zap**: ~3-7ns per log call
- **stdlog**: ~8-12ns per log call

Performance characteristics:
- Zero allocations in hot path when handler supports it
- Lock-free level checks
- Context cancellation short-circuits before allocation

Comprehensive benchmarks will be published when all handlers are finalized.

## Documentation

- [Handler Comparison](docs/HANDLERS.md) - Feature matrix, performance, use cases
- [Architecture](docs/ARCHITECTURE.md) - System design, handler contract
- [Handler Development](docs/HANDLER_DEVELOPMENT.md) - Implement custom handlers
- [Compatibility](docs/COMPATIBILITY.md) - Version policy, deprecation rules
- [Troubleshooting](docs/TROUBLESHOOTING.md) - Common issues, debugging

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Code standards and review process
- How to add new handlers
- Testing requirements
- Pull request guidelines

## Stability

- **Core API**: Stable, follows semantic versioning
- **Handler API**: Stable, follows semantic versioning
- **Handler Implementations**: slog, zap, stdlog are stable; others under rewrite

Breaking changes only on major version bumps. See [COMPATIBILITY.md](docs/COMPATIBILITY.md) for details.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Alternatives

Why not use these directly?

- **slog**: Requires Go 1.21+, limited to stdlib features
- **zap**: High-performance but complex configuration
- **zerolog**: Zero-allocation focus may not suit all use cases
- **logrus**: Popular but slower, hooks add complexity

unilog lets you choose the best backend for each component without rewriting application code.