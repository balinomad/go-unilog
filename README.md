[![GoDoc](https://pkg.go.dev/badge/github.com/balinomad/go-unilog?status.svg)](https://pkg.go.dev/github.com/balinomad/go-unilog?tab=doc)
[![GoMod](https://img.shields.io/github/go-mod/go-version/balinomad/go-unilog)](https://github.com/balinomad/go-unilog)
[![Size](https://img.shields.io/github/languages/code-size/balinomad/go-unilog)](https://github.com/balinomad/go-unilog)
[![License](https://img.shields.io/github/license/balinomad/go-unilog)](./LICENSE)
[![Go](https://github.com/balinomad/go-unilog/actions/workflows/go.yml/badge.svg)](https://github.com/balinomad/go-unilog/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/balinomad/go-unilog)](https://goreportcard.com/report/github.com/balinomad/go-unilog)
[![codecov](https://codecov.io/github/balinomad/go-unilog/graph/badge.svg?token=L1K68IIN51)](https://codecov.io/github/balinomad/go-unilog)

# unilog

*A lightweight and idiomatic Go library to offer a unified logger interface.*

## 🤔 Why unilog?

In a large Go application or a reusable library, coupling directly to a specific logging library creates vendor lock-in. If you decide to switch from `zerolog` to `slog`, you would have to refactor every logging call site in your project.

`go-unilog` solves this by providing a standardized `Logger` interface. Your application code only interacts with this interface. Behind the scenes, an adapter translates calls from the `unilog.Logger` interface to the specific API of your chosen logging library. This means you can swap out the underlying logger at any time, in one central place, without rewriting your application code.

Perfect for use in:

  - **Libraries**: Allow consumers of your library to integrate your module's logs seamlessly into their application's existing logging infrastructure.
  - **Large Applications & Microservices**: Enforce a consistent logging interface across your entire codebase, even if different services or teams have different preferences for logging libraries.
  - **Projects in Flux**: Start with the simple built-in logger and switch to a more powerful, structured logger like `zap` or `slog` as your project's needs evolve, without the headache of a major refactor.

## ✨ Features

  - **Unified Interface**: A single, easy-to-use `Logger` interface for all your logging needs.
  - **Adapter-Based**: Pluggable adapters for popular logging libraries including `slog`, `zap`, `logrus`, `zerolog`, and the standard `log` package.
  - **Structured Logging**: Encourages best practices with structured, key-value pair logging.
  - **Leveled Logging**: Supports standard log levels: `Debug`, `Info`, `Warn`, `Error`, `Critical`, and `Fatal`.
  - **Context-Aware**: Pass `context.Context` through your application for request-scoped logging.
  - **Minimal Core Dependencies**: The core `unilog` API has zero third-party dependencies, making it safe to use in libraries. The default fallback logger has one small internal dependency for thread-safe writes.
  - **Panic-Safe**: Includes a fallback logger that ensures `unilog.Info()` and other package-level functions never panic, even if no logger is configured.

## 📌 Installation

```bash
go get github.com/balinomad/go-unilog@latest
```

## 🚀 Basic Usage

Placeholder for an example with the built-in `slog` adapter.

```go
package main

import (
	"context"
	"errors"

	"github.com/balinomad/go-unilog"
	"github.com/balinomad/go-unilog/adapter/slog"
)

func main() {
	ctx := context.Background()
	logger := slog.New(WithOutput(os.Stderr), WithFormat(slog.FormatJSON), WithCaller(true))

	unilog.SetDefault(logger)

	err := doSomething(ctx)
	if err != nil {
		unilog.Error(ctx, "failed to do something", unilog.Error(err))
		return
	}

	unilog.Info(ctx, "did something")
}

func doSomething(ctx context.Context) error {
	unilog.Debug(ctx, "trying to do something")
	// simulate some work
	time.Sleep(100 * time.Millisecond)
	return nil
}
```

## 📘 API Reference

### The `Logger` Interface

| Function | Description |
| -------- | ----------- |
| `Logger.Log(ctx, level, msg, ...any)` | The generic logging method. All other level methods call this. |
| `Logger.Trace(ctx, msg, ...any)` | Logs a message at the TRACE level. |
| `Logger.Debug(ctx, msg, ...any)` | Logs a message at the DEBUG level. |
| `Logger.Info(ctx, msg, ...any)` | Logs a message at the INFO level. |
| `Logger.Warn(ctx, msg, ...any)` | Logs a message at the WARN level. |
| `Logger.Error(ctx, msg, ...any)` | Logs a message at the ERROR level. |
| `Logger.Critical(ctx, msg, ...any)` | Logs a message at the CRITICAL level. |
| `Logger.Fatal(ctx, msg, ...any)` | Logs a message at the FATAL level and exits the process. |
| `Logger.Panic(ctx, msg, ...any)` | Logs a message at the PANIC level and panics. |
| `Logger.Enabled(level)` | Returns true if the given log level is enabled. |
| `Logger.With(...any)` | Returns a new logger with the provided key-value pairs always included. |
| `Logger.WithGroup(name)` | Returns a new logger that starts a new group with the provided name. |

### Package-level Functions

| Function | Description |
| -------- | ----------- |
| `unilog.Default()` | Returns the default logger. |
| `unilog.SetDefault(logger)` | Sets the default logger. |
| `unilog.WithLogger(ctx, logger)` | Returns a new context with the provided logger. |
| `unilog.LoggerFromContext(ctx)` | Returns the logger from the context. |
| `unilog.Log(ctx, level, msg, ...any)` | Logs a message at the given level using the default logger. |
| `unilog.Trace(ctx, msg, ...any)` | Logs a message at the TRACE level using the default logger. |
| `unilog.Info(ctx, msg, ...any)` | Logs a message at the INFO level using the default logger. |
| `unilog.Debug(ctx, msg, ...any)` | Logs a message at the DEBUG level using the default logger. |
| `unilog.Warn(ctx, msg, ...any)` | Logs a message at the WARN level using the default logger. |
| `unilog.Error(ctx, msg, ...any)` | Logs a message at the ERROR level using the default logger. |
| `unilog.Critical(ctx, msg, ...any)` | Logs a message at the CRITICAL level using the default logger. |
| `unilog.Fatal(ctx, msg, ...any)` | Logs a message at the FATAL level using the default logger and exits the process. |
| `unilog.Panic(ctx, msg, ...any)` | Logs a message at the PANIC level using the default logger and panics. |
| `unilog.LogWithSkip(ctx, level, msg, skip, ...any)` | Logs a message at the given level using the default logger, skipping the given number of stack frames if supported. |

### `Configurator` for Loggers with Dynamic Reconfiguration

| Function | Description |
| -------- | ----------- |
| `Configurator.SetLevel(level)` | Sets the minimum enabled log level for a logger. |
| `Configurator.SetOutput(writer)` | Changes the log output destination for a logger. |

### `CallerSkipper` for Loggers with Advanced Caller Reporting

| Function | Description |
| -------- | ----------- |
| `CallerSkipper.LogWithSkip(ctx, level, msg, skip, ...any)` | Logs a message at the given level, skipping the given number of stack frames. |
| `CallerSkipper.CallerSkip()` | Returns the current number of stack frames skipped when stack traces are logged. |
| `CallerSkipper.WithCallerSkip(skip)` | Returns a new `Logger` with the caller skip set. |
| `CallerSkipper.WithCallerSkipDelta(delta)` | Returns a new `Logger` with caller skip adjusted by delta. |

### `Cloner` for Loggers with Deep Cloning

| Function | Description |
| -------- | ----------- |
| `Cloner.Clone()` | Returns a deep copy of the logger. |

### `Syncer` for Loggers with Buffered Logging

| Function | Description |
| -------- | ----------- |
| `Syncer.Sync()` | Flushes any buffered log entries. |

### `LogLevel` is a Type Representing Log Severity Levels

Levels: `TraceLevel`, `DebugLevel`, `InfoLevel`, `WarnLevel`, `ErrorLevel`, `CriticalLevel`, `FatalLevel`, and `PanicLevel`

| Function | Description |
| -------- | ----------- |
| `LogLevel.String()` | Returns a human-readable representation of the log level. |
| `unilog.ParseLevel(levelStr)` | Converts a string to a `LogLevel`. |
| `unilog.IsValidLogLevel(level)` | Returns true if the given log level is valid. |
| `unilog.ValidateLogLevel(level)` | Returns an error if the given log level is invalid. |

## 🔧 Advanced Example

### Implementing a Custom Adapter

The true power of `go-unilog` is its ability to adapt to any logging library, including your own custom or in-house solutions. To do this, you simply need to create a new adapter that satisfies the `unilog.Logger` interface.

Below is a complete, self-contained example of creating an adapter for a fictional, minimalistic logger called `minlog`.

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/balinomad/go-unilog"
)

// --- Step 1: Define Your Custom Logger ---

// This is the logger we want to adapt. It's a simple logger that doesn't
// conform to any standard interface.
type minlog struct {
	mu sync.Mutex
	w  io.Writer
}

// Log writes a simple formatted string.
func (l *minlog) Log(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.w, msg)
}

// --- Step 2: Create the Adapter ---

// The adapter will wrap `minlog` and implement the `unilog.Logger` interface.
type minlogAdapter struct {
	logger *minlog
	level  unilog.LogLevel
	fields []any // For storing fields from With()
}

// NewMinlogAdapter creates a new adapter.
func NewMinlogAdapter(logger *minlog, level unilog.LogLevel) *minlogAdapter {
	return &minlogAdapter{
		logger: logger,
		level:  level,
	}
}

// --- Step 3: Implement the unilog.Logger Interface ---

// Log is the core method that translates unilog calls to minlog calls.
func (a *minlogAdapter) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	if !a.Enabled(level) {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s", level, msg))

	// Combine fields from With() and the current call.
	allFields := append(a.fields, keyValues...)

	for i := 0; i < len(allFields); i += 2 {
		key := allFields[i]
		var val any = "(no value)"
		if i+1 < len(allFields) {
			val = allFields[i+1]
		}
		sb.WriteString(fmt.Sprintf(" %v=%v", key, val))
	}

	a.logger.Log(sb.String())

	switch level {
	case unilog.FatalLevel:
		os.Exit(1)
	case unilog.PanicLevel:
		panic(msg)
	}
}

// Enabled checks if a given log level is active.
func (a *minlogAdapter) Enabled(level unilog.LogLevel) bool {
	return level >= a.level
}

// With returns a new adapter instance with the added fields.
func (a *minlogAdapter) With(keyValues ...any) unilog.Logger {
	newAdapter := &minlogAdapter{
		logger: a.logger,
		level:  a.level,
		// Create a new slice and copy fields to ensure immutability.
		fields: make([]any, len(a.fields)+len(keyValues)),
	}
	copy(newAdapter.fields, a.fields)
	copy(newAdapter.fields[len(a.fields):], keyValues)
	return newAdapter
}

// WithGroup is a simplified implementation for this example.
func (a *minlogAdapter) WithGroup(name string) unilog.Logger {
	// A real implementation would prefix subsequent keys.
	// For this example, we'll just add a group field.
	return a.With("group", name)
}

// Implement the convenience methods by calling Log.
func (a *minlogAdapter) Trace(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.TraceLevel, msg, keyValues...)
}
func (a *minlogAdapter) Debug(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.DebugLevel, msg, keyValues...)
}
func (a *minlogAdapter) Info(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.InfoLevel, msg, keyValues...)
}
func (a *minlogAdapter) Warn(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.WarnLevel, msg, keyValues...)
}
func (a *minlogAdapter) Error(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.ErrorLevel, msg, keyValues...)
}
func (a *minlogAdapter) Critical(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.CriticalLevel, msg, keyValues...)
}
func (a *minlogAdapter) Fatal(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.FatalLevel, msg, keyValues...)
}
func (a *minlogAdapter) Panic(ctx context.Context, msg string, keyValues ...any) {
	a.Log(ctx, unilog.PanicLevel, msg, keyValues...)
}

// --- Step 4: Use Your New Adapter ---

func main() {
	// Instantiate your custom logger.
	customLogger := &minlog{w: os.Stdout}

	// Create an instance of your new adapter.
	adapter := NewMinlogAdapter(customLogger, unilog.InfoLevel)

	// Set it as the default unilog logger.
	unilog.SetDefault(adapter)

	// All package-level calls now go through your adapter.
	fmt.Println("--- Logging with the custom adapter ---")
	unilog.Info(context.Background(), "User logged in", "user_id", 123)
	unilog.Warn(context.Background(), "API limit approaching", "remaining", 5)

	// Demonstrate the With() method.
	requestLogger := unilog.Default().With("request_id", "abc-123")
	requestLogger.Info(context.Background(), "Processing request")
	requestLogger.Error(context.Background(), "Failed to fetch data", "url", "/api/data")
}

// Expected output:
// --- Logging with the custom adapter ---
// [INFO] User logged in user_id=123
// [WARN] API limit approaching remaining=5
// [INFO] Processing request request_id=abc-123
// [ERROR] Failed to fetch data request_id=abc-123 url=/api/data
```

## 🧪 Testing

Run tests with:
```bash
go test -v ./...
```

Run benchmarks with:
```bash
go test -bench=. -benchmem ./...
```

## ⚖️ License

MIT License — see `LICENSE` file for details.
