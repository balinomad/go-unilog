package unilog

// This file exports thin wrappers around unexported helpers so unit tests
// in package unilog_test can exercise their behavior without moving tests
// into the package under test. Keep wrappers minimal and stable.

// XAtomicWriterError calls the unexported atomicWriterError with the
// provided underlying error and returns the resulting error.
func XAtomicWriterError(u error) error { return atomicWriterError(u) }

// XOptionError calls the unexported optionError with the provided
// underlying error and returns the resulting error.
func XOptionError(u error) error { return optionError(u) }

// XInvalidFormatError calls the unexported invalidFormatError with the
// provided format and accepted list and returns the resulting error.
func XInvalidFormatError(format string, accepted []string) error {
	return invalidFormatError(format, accepted)
}

// XLoggerKey is the context key used to store loggers.
var XLoggerKey = loggerKey

// XNewFallbackLogger creates a fallback logger for testing.
var XNewFallbackLogger = newFallbackLogger

// XFallbackLogger exposes the fallback logger type for testing.
type XFallbackLogger = fallbackLogger
