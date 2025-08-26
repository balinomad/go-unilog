package unilog

import (
	"errors"
	"testing"
)

func TestErrorTypes(t *testing.T) {
	t.Run("Static Errors", func(t *testing.T) {
		if ErrNilWriter.Error() != "writer cannot be nil" {
			t.Errorf("ErrNilWriter has wrong message: %q", ErrNilWriter.Error())
		}
		if ErrInvalidSourceSkip.Error() != "source skip must be non-negative" {
			t.Errorf("ErrInvalidSourceSkip has wrong message: %q", ErrInvalidSourceSkip.Error())
		}
	})

	t.Run("Functional Errors", func(t *testing.T) {
		baseErr := errors.New("base error")

		tests := []struct {
			name               string
			errFunc            func(error) error
			expectedMsg        string
			expectedWrappedErr error
		}{
			{
				name:               "ErrAtomicWriterFail",
				errFunc:            func(_ error) error { return ErrAtomicWriterFail(baseErr) },
				expectedMsg:        "failed to create atomic writer: base error",
				expectedWrappedErr: baseErr,
			},
			{
				name:               "ErrFailedOption",
				errFunc:            func(_ error) error { return ErrFailedOption(baseErr) },
				expectedMsg:        "failed to apply option: base error",
				expectedWrappedErr: baseErr,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.errFunc(baseErr)
				if err.Error() != tt.expectedMsg {
					t.Errorf("error message = %q, want %q", err.Error(), tt.expectedMsg)
				}
				if !errors.Is(err, tt.expectedWrappedErr) {
					t.Errorf("error does not wrap the base error")
				}
			})
		}
	})

	t.Run("ErrInvalidFormat", func(t *testing.T) {
		err := ErrInvalidFormat("xml", "json", "text")
		expected := `invalid format: "xml", must be one of [json text]`
		if err.Error() != expected {
			t.Errorf("ErrInvalidFormat() = %q, want %q", err.Error(), expected)
		}
	})
}
