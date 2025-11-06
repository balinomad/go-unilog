# Handler Implementations

## Overview

Handlers adapt third-party logging libraries to the `handler.Handler` interface. Each handler provides:

- **Level mapping**: Translates unilog levels to backend-specific levels
- **Attribute conversion**: Converts `[]Attr` to backend format
- **Optional features**: Implements additional interfaces as supported

## Interface Support Matrix

| Handler  | Chainer | Configurator | Syncer | Cloner | Notes                              |
|----------|---------|--------------|--------|--------|------------------------------------|
| slog     | ✓       | ✓            | ✗      | ✗      | Standard library, no sync needed   |
| zap      | ✓       | ✓            | ✓      | ✓      | High performance, all features     |
| zerolog  | ✓       | ✓            | ✗      | ✗      | Zero-allocation design             |
| logrus   | ✓       | ✓            | ✗      | ✗      | Structured logging with hooks      |
| log15    | ✓       | ✓            | ✗      | ✗      | Terminal-friendly formatting       |
| stdlog   | ✓       | ✓            | ✗      | ✗      | Stdlib `log` with structured attrs |

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

Not every handler supports context propagation.

## Performance Characteristics

## Caller Skip Behavior

## Creating Custom Handlers

See [`docs/CONTRIBUTING.md`](CONTRIBUTING.md) for implementation guide.