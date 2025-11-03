package unilog

import (
	"errors"
	"fmt"
)

// Sentinel errors for common conditions
var (
	ErrAtomicWriterFail  = errors.New("failed to create atomic writer")
	ErrFailedOption      = errors.New("failed to apply option")
	ErrInvalidFormat     = errors.New("invalid format")
	ErrInvalidSourceSkip = errors.New("source skip must be non-negative")
	ErrNilWriter         = errors.New("writer cannot be nil")
)

// atomicWriterError returns an error with ErrAtomicWriterFail.
func atomicWriterError(err error) error {
	return fmt.Errorf("%w: %w", ErrAtomicWriterFail, err)
}

// optionError returns an error with ErrFailedOption.
func optionError(err error) error {
	return fmt.Errorf("%w: %w", ErrFailedOption, err)
}

// invalidFormatError returns an error with ErrInvalidFormat.
func invalidFormatError(format string, accepted []string) error {
	return fmt.Errorf("%w: %q, must be one of %v", ErrInvalidFormat, format, accepted)
}
