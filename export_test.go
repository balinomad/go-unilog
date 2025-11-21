package unilog

import "github.com/balinomad/go-unilog/handler"

// This file exports thin wrappers around unexported helpers so unit tests
// in package unilog_test can exercise their behavior without moving tests
// into the package under test. Keep wrappers minimal and stable.

// XLoggerKey is the context key used to store loggers.
var XLoggerKey = loggerKey

// XNewFallbackLogger creates a fallback logger for testing.
var XNewFallbackLogger = newFallbackLogger

// XNewSimpleFallbackLogger creates a simple fallback logger for testing.
var XNewSimpleFallbackLogger = newSimpleFallbackLogger

// XFallbackLogger exposes the fallback logger type for testing.
type XFallbackLogger = fallbackLogger

// XNewLogger exposes the internal newLogger constructor for invariant testing.
var XNewLogger = newLogger

// XInternalSkipFrames exposes the call stack depth constant to ensure tests
// remain robust if internal implementation depth changes.
const XInternalSkipFrames = internalSkipFrames

// XLoggerHandler retrieves the underlying handler from the logger implementation.
// This is required for white-box testing of delegation logic in unilog_test.
func XLoggerHandler(l Logger) handler.Handler {
	if impl, ok := l.(*logger); ok {
		return impl.h
	}
	return nil
}
