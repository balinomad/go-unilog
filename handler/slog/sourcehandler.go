package slog

import (
	"context"
	"log/slog"
	"runtime"
	"strconv"
	"sync/atomic"
)

// SourceHandler wraps another slog.Handler and injects source info with dynamic skip control.
//
// This is an alternative to log/slog's AddSource, which does not support dynamic skipping.
type SourceHandler struct {
	next      slog.Handler
	skip      *atomic.Int32
	sourceKey string
}

// Ensures SourceHandler implements slog.Handler.
var _ slog.Handler = (*SourceHandler)(nil)

// NewSourceHandler creates a new SourceHandler.
//   - next: the wrapped handler (JSONHandler, TextHandler, etc.).
//   - skip: atomic int storing the effective skip value (user + internal frames).
//   - sourceKey: optional custom key for the injected source field (defaults to "source").
func NewSourceHandler(next slog.Handler, skip *atomic.Int32, sourceKey ...string) *SourceHandler {
	key := slog.SourceKey
	if len(sourceKey) > 0 && sourceKey[0] != "" {
		key = sourceKey[0]
	}

	return &SourceHandler{
		next:      next,
		skip:      skip,
		sourceKey: key,
	}
}

// Enabled delegates to the wrapped handler.
func (h *SourceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle injects the source file:line and delegates to the wrapped handler.
func (h *SourceHandler) Handle(ctx context.Context, r slog.Record) error {
	skip := int(h.skip.Load())
	if _, file, line, ok := runtime.Caller(skip); ok {
		r.AddAttrs(slog.String(h.sourceKey, file+":"+strconv.Itoa(line)))
	}

	return h.next.Handle(ctx, r)
}

// WithAttrs returns a copy of the SourceHandler with attrs added.
func (h *SourceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SourceHandler{
		next:      h.next.WithAttrs(attrs),
		skip:      h.skip,
		sourceKey: h.sourceKey,
	}
}

// WithGroup returns a copy of the SourceHandler with group added.
func (h *SourceHandler) WithGroup(name string) slog.Handler {
	return &SourceHandler{
		next:      h.next.WithGroup(name),
		skip:      h.skip,
		sourceKey: h.sourceKey,
	}
}
