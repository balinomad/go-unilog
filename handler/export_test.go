package handler

// This file exports thin wrappers around unexported helpers so unit tests
// in package handler_test can exercise their behavior without moving tests
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

// XInvalidLogLevelError calls the unexported invalidLogLevelError with the
// provided LogLevel and returns the resulting error.
func XInvalidLogLevelError(level LogLevel) error {
	return invalidLogLevelError(level)
}
