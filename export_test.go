package unilog

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
