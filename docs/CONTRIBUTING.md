# Contributing a Handler

## Implementation Checklist

1. Create package under `handler/<name>/`
2. Implement `handler.Handler` interface:
```go
   type myHandler struct {
       base *handler.BaseHandler
       // backend-specific fields
   }

   func (h *myHandler) Handle(ctx context.Context, r *handler.Record) error {
       // Convert Record to backend format
   }

   func (h *myHandler) Enabled(level handler.LogLevel) bool {
       return h.base.Enabled(level)
   }
```
3. Implement optional interfaces as supported (Chainer, Configurator, etc.)
4. Create `New()` constructor returning `unilog.Logger`
5. Add compliance test:
```go
   func TestCompliance(t *testing.T) {
       handler.ComplianceTest(t, func() (handler.Handler, error) {
           return newMyHandler(...)
       })
   }
```
6. Update `docs/HANDLERS.md` with interface support and level mapping

## Example

See [`handler/slog/slog.go`](../handler/slog/slog.go) for reference implementation.
