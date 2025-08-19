package unilog

import (
	"errors"
	"fmt"
)

var (
	ErrNilWriter         = errors.New("writer cannot be nil")
	ErrAtomicWriterFail  = func(e error) error { return fmt.Errorf("failed to create atomic writer: %w", e) }
	ErrInvalidSourceSkip = errors.New("source skip must be non-negative")
	ErrFailedOption      = func(e error) error { return fmt.Errorf("failed to apply option: %w", e) }
	ErrInvalidFormat     = func(f string, accepted ...string) error {
		return fmt.Errorf("invalid format: %q, must be one of %v", f, accepted)
	}
)
