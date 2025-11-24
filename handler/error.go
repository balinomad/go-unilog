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

// NewAtomicWriterError returns an error wrapping ErrAtomicWriterFail.
func NewAtomicWriterError(err error) error {
	return errors.Join(ErrAtomicWriterFail, err)
}

// NewOptionApplyError returns an error wrapping ErrOptionApplyFailed.
func NewOptionApplyError(option string, err error) error {
	return errors.Join(fmt.Errorf("%s: %w", option, ErrOptionApplyFailed), err)
}

// NewInvalidFormatError returns an error wrapping ErrInvalidFormat.
func NewInvalidFormatError(format string, accepted []string) error {
	return fmt.Errorf("%w: got %q, expected one of %v", ErrInvalidFormat, format, accepted)
}

// NewInvalidLogLevelError returns an error wrapping ErrInvalidLogLevel.
func NewInvalidLogLevelError(level LogLevel) error {
	return fmt.Errorf("%w: got %d, must be in range [%d, %d]", ErrInvalidLogLevel, level, MinLevel, MaxLevel)
}
