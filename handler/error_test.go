package handler_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

func TestErrorHelpers(t *testing.T) {
	t.Parallel()

	// Define stable underlying errors for testing
	errUnderlyingAtomic := io.ErrClosedPipe
	errUnderlyingOption := io.ErrUnexpectedEOF

	tests := []struct {
		name           string
		err            error    // The error returned from the wrapper
		wantErr        error    // The sentinel error to check for
		wantUnderlying error    // The underlying error to check for
		wantContains   []string // Substrings the error message must contain
	}{
		{
			name:           "atomic_writer_error",
			err:            handler.XAtomicWriterError(errUnderlyingAtomic),
			wantErr:        handler.ErrAtomicWriterFail,
			wantUnderlying: errUnderlyingAtomic,
			wantContains: []string{
				handler.ErrAtomicWriterFail.Error(),
				errUnderlyingAtomic.Error(),
			},
		},
		{
			name:           "option_error",
			err:            handler.XOptionError(errUnderlyingOption),
			wantErr:        handler.ErrFailedOption,
			wantUnderlying: errUnderlyingOption,
			wantContains: []string{
				handler.ErrFailedOption.Error(),
				errUnderlyingOption.Error(),
			},
		},
		{
			name:    "invalid_format_error",
			err:     handler.XInvalidFormatError("foo", []string{"bar", "baz"}),
			wantErr: handler.ErrInvalidFormat,
			wantContains: []string{
				handler.ErrInvalidFormat.Error(),
				`"foo"`,
				"[bar baz]",
			},
		},
		{
			name:    "invalid_format_error_empty_accepted",
			err:     handler.XInvalidFormatError("foo", nil),
			wantErr: handler.ErrInvalidFormat,
			wantContains: []string{
				handler.ErrInvalidFormat.Error(),
				`"foo"`,
				"[]",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err == nil {
				t.Fatal("got nil error, expected non-nil")
			}

			// Check that the error wraps the main sentinel error
			if !errors.Is(tt.err, tt.wantErr) {
				t.Errorf("errors.Is(err, wantErr) = false, want true (wantErr: %v, err: %v)", tt.wantErr, tt.err)
			}

			// Check that the error wraps the underlying error, if specified
			if tt.wantUnderlying != nil {
				if !errors.Is(tt.err, tt.wantUnderlying) {
					t.Errorf("errors.Is(err, wantUnderlying) = false, want true (wantUnderlying: %v, err: %v)", tt.wantUnderlying, tt.err)
				}
			}

			// Check the error message content
			errMsg := tt.err.Error()
			for _, substr := range tt.wantContains {
				if !strings.Contains(errMsg, substr) {
					t.Errorf("error message %q does not contain expected substring %q", errMsg, substr)
				}
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"ErrAtomicWriterFail", handler.ErrAtomicWriterFail, "failed to create atomic writer"},
		{"ErrFailedOption", handler.ErrFailedOption, "failed to apply option"},
		{"ErrInvalidFormat", handler.ErrInvalidFormat, "invalid format"},
		{"ErrInvalidSourceSkip", handler.ErrInvalidSourceSkip, "source skip must be non-negative"},
		{"ErrNilWriter", handler.ErrNilWriter, "writer cannot be nil"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err == nil {
				t.Fatal("sentinel error is nil")
			}
			if msg := tt.err.Error(); msg != tt.wantMsg {
				t.Errorf("error message = %q, want %q", msg, tt.wantMsg)
			}
			// Check that sentinel errors do not wrap anything
			if unwrapped := errors.Unwrap(tt.err); unwrapped != nil {
				t.Errorf("sentinel error unexpectedly unwraps to: %v", unwrapped)
			}
		})
	}
}
