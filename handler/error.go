package handler

import (
	"errors"
	"fmt"
)

// Sentinel errors for common conditions
var (
	ErrInvalidLogLevel   = errors.New("invalid log level")
	ErrAtomicWriterFail  = errors.New("failed to create atomic writer")
	ErrOptionApplyFailed = errors.New("failed to apply option")
	ErrInvalidFormat     = errors.New("invalid format")
	ErrInvalidSourceSkip = errors.New("source skip must be non-negative")
	ErrNilWriter         = errors.New("writer cannot be nil")
)

// NewAtomicWriterError returns an error with ErrAtomicWriterFail.
func NewAtomicWriterError(err error) error {
	return fmt.Errorf("%w: %w", ErrAtomicWriterFail, err)
}

// NewOptionApplyError returns an error with ErrOptionApplyFailed.
func NewOptionApplyError(err error) error {
	return fmt.Errorf("%w: %w", ErrOptionApplyFailed, err)
}

// NewInvalidFormatError returns an error with ErrInvalidFormat.
func NewInvalidFormatError(format string, accepted []string) error {
	return fmt.Errorf("%w: %q, must be one of %v", ErrInvalidFormat, format, accepted)
}

// NewInvalidLogLevelError returns an error with ErrInvalidLogLevel when a LogLevel is out of range.
func NewInvalidLogLevelError(level LogLevel) error {
	return fmt.Errorf("%w: %d, must be between %d and %d", ErrInvalidLogLevel, level, MinLevel, MaxLevel)
}
