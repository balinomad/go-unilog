package unilog

// Exported for testing only. These symbols are only available during tests
// and do not pollute the public API.

// LoggerKey is the context key used to store loggers.
var LoggerKey = loggerKey

// NewFallbackLogger creates a fallback logger for testing.
var NewFallbackLogger = newFallbackLogger

// FallbackLogger exposes the fallback logger type for testing.
type FallbackLogger = fallbackLogger
