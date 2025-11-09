package handler_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

func TestNewErrorWrappers(t *testing.T) {
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
			name:           "atomic writer error",
			err:            handler.NewAtomicWriterError(errUnderlyingAtomic),
			wantErr:        handler.ErrAtomicWriterFail,
			wantUnderlying: errUnderlyingAtomic,
			wantContains: []string{
				handler.ErrAtomicWriterFail.Error(),
				errUnderlyingAtomic.Error(),
			},
		},
		{
			name:           "option error",
			err:            handler.NewOptionApplyError("myOption", errUnderlyingOption),
			wantErr:        handler.ErrOptionApplyFailed,
			wantUnderlying: errUnderlyingOption,
			wantContains: []string{
				handler.ErrOptionApplyFailed.Error(),
				errUnderlyingOption.Error(),
			},
		},
		{
			name:    "invalid format error",
			err:     handler.NewInvalidFormatError("foo", []string{"bar", "baz"}),
			wantErr: handler.ErrInvalidFormat,
			wantContains: []string{
				handler.ErrInvalidFormat.Error(),
				`"foo"`,
				"[bar baz]",
			},
		},
		{
			name:    "invalid format error empty accepted",
			err:     handler.NewInvalidFormatError("foo", nil),
			wantErr: handler.ErrInvalidFormat,
			wantContains: []string{
				handler.ErrInvalidFormat.Error(),
				`"foo"`,
				"[]",
			},
		},
		{
			name:    "invalid log level error below min",
			err:     handler.NewInvalidLogLevelError(handler.MinLevel - 1),
			wantErr: handler.ErrInvalidLogLevel,
			wantContains: []string{
				handler.ErrInvalidLogLevel.Error(),
				fmt.Sprintf("%d", handler.MinLevel-1),
				fmt.Sprintf("%d", handler.MinLevel),
				fmt.Sprintf("%d", handler.MaxLevel),
			},
		},
		{
			name:    "invalid log level error above max",
			err:     handler.NewInvalidLogLevelError(handler.MaxLevel + 1),
			wantErr: handler.ErrInvalidLogLevel,
			wantContains: []string{
				handler.ErrInvalidLogLevel.Error(),
				fmt.Sprintf("%d", handler.MaxLevel+1),
				fmt.Sprintf("%d", handler.MinLevel),
				fmt.Sprintf("%d", handler.MaxLevel),
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
				t.Errorf("errors.Is(err, sentinel) = false, want true (sentinel: %v, err: %v)", tt.wantErr, tt.err)
			}

			// Check that the error wraps the underlying error, if specified
			if tt.wantUnderlying != nil {
				if !errors.Is(tt.err, tt.wantUnderlying) {
					t.Errorf("errors.Is(err, underlying) = false, want true (underlying: %v, err: %v)", tt.wantUnderlying, tt.err)
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
		{"ErrInvalidLogLevel", handler.ErrInvalidLogLevel, "invalid log level"},
		{"ErrAtomicWriterFail", handler.ErrAtomicWriterFail, "failed to create atomic writer"},
		{"ErrOptionApplyFailed", handler.ErrOptionApplyFailed, "failed to apply option"},
		{"ErrInvalidFormat", handler.ErrInvalidFormat, "invalid format"},
		{"ErrInvalidSourceSkip", handler.ErrInvalidSourceSkip, "source skip must be non-negative"},
		{"ErrNilWriter", handler.ErrNilWriter, "writer cannot be nil"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err == nil {
				t.Fatalf("sentinel %s is nil", tt.name)
			}
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Fatalf("sentinel %s message = %q; want %q", tt.name, got, tt.wantMsg)
			}
			// Check that sentinel errors do not wrap anything
			if unwrapped := errors.Unwrap(tt.err); unwrapped != nil {
				t.Fatalf("sentinel %s unexpectedly unwraps to: %v", tt.name, unwrapped)
			}
		})
	}
}
