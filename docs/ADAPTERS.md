# ADAPTERS

## Feature Matrix

| `unilog` Adapter | Stdlib | Slog | Zap | Zerolog | Logrus | Log15 |
|------------------|--------|------|-----|---------|--------|-------|
| **Configurator** | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **StackLogger**  | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **Cloner**       | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ |
| **Syncer**       | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ |


| Adapter  | Configurator | CallerSkipper | Syncer | Notes                     |
|----------|--------------|---------------|--------|---------------------------|
| slog     | ✓ | ✓ | ✗ | |
| zap      | ✓ | ✓ | ✓ | |
| logrus   | ✓ | ✓ | ✗ | |
| zerolog  | ✓ | ✓ | ✗ | |
| log15    | ✓ | ✓ | ✗ | |
| stdlog   | ✓ | ✓ | ✗ | |

## Log Level Mapping

Not every adapter has the same log levels. To main tain consistent behavior, we need to map unilog levels to the nearest semantic equivalent in each adapter.

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

Not every adapter supports context propagation.

## Performance Characteristics

## Caller Skip Behavior