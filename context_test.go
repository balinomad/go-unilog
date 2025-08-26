package unilog

import (
	"context"
	"io"
	"testing"
)

func TestContext(t *testing.T) {
	t.Run("LoggerFromContext with empty context", func(t *testing.T) {
		ctx := context.Background()
		logger, ok := LoggerFromContext(ctx)
		if ok {
			t.Error("LoggerFromContext() ok = true, want false")
		}
		if logger != nil {
			t.Errorf("LoggerFromContext() logger = %v, want nil", logger)
		}
	})

	t.Run("WithLogger and LoggerFromContext success", func(t *testing.T) {
		ctx := context.Background()
		expectedLogger, _ := newFallbackLogger(io.Discard, LevelInfo)
		ctxWithLogger := WithLogger(ctx, expectedLogger)

		if ctxWithLogger == ctx {
			t.Fatal("WithLogger returned the same context")
		}

		retrievedLogger, ok := LoggerFromContext(ctxWithLogger)
		if !ok {
			t.Error("LoggerFromContext() ok = false, want true")
		}
		if retrievedLogger != expectedLogger {
			t.Errorf("LoggerFromContext() got %v, want %v", retrievedLogger, expectedLogger)
		}
	})

	t.Run("LoggerFromContext with wrong type in value", func(t *testing.T) {
		ctx := context.Background()
		ctxWithWrongType := context.WithValue(ctx, loggerKey, "not a logger")
		logger, ok := LoggerFromContext(ctxWithWrongType)
		if ok {
			t.Error("LoggerFromContext() ok = true, want false")
		}
		if logger != nil {
			t.Errorf("LoggerFromContext() logger = %v, want nil", logger)
		}
	})
}
