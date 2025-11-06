# Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                    User Application                     │
└───────────────────────────┬─────────────────────────────┘
                            │ calls
                            ▼
                  User-facing interface
┌─────────────────────────────────────────────────────────┐
│                      unilog.Logger                      │
├─────────────────────────────────────────────────────────┤
│  - Trace/Debug/Info/... (convenience methods)           │
│  - With/WithGroup (chaining)                            │
│  - Log(ctx, level, msg, keyValues)                      │
└───────────────────────────┬─────────────────────────────┘
                            │ wraps
                            ▼
                  Wrapper implementation
┌─────────────────────────────────────────────────────────┐
│                   internal.loggerImpl                   │
├─────────────────────────────────────────────────────────┤
│  - Implements Logger interface                          │
│  - Builds Record from call parameters                   │
│  - Calculates caller PC (for skip support)              │
│  - Checks context cancellation                          │
│  - Type-asserts optional interfaces                     │
└───────────────────────────┬─────────────────────────────┘
                            │ delegates to
                            ▼
                    Adapter interface
┌─────────────────────────────────────────────────────────┐
│                    handler.Handler                      │
├─────────────────────────────────────────────────────────┤
│  - Handle(ctx, record)                                  │
│  - Enabled(level)                                       │
│  + Optional: Chainer, Configurator, Lifecycle, Cloner   │
└───────────────────────────┬─────────────────────────────┘
                            │ implements
                            ▼
                    Concrete adapters
       ┌────────────────────┴───┬──────────────────┐
       ▼                        ▼                  ▼
┌─────────────┐          ┌─────────────┐    ┌─────────────┐
│ slogHandler │          │ zapHandler  │    │ zeroHandler │
│ (stdlib)    │   ...    │ (zap)       │    │ (zerolog)   │
└──────┬──────┘          └──────┬──────┘    └──────┬──────┘
       │                        │                  │
       └────────────────────┬───┴──────────────────┘
                            │ uses
                            ▼
                  Third-party libraries
              ┌────────────────────────────┐
              │   Backend Logger           │
              │ (slog, zap, zerolog, etc.) │
              └────────────────────────────┘
```
