package handler_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

// mockHandler is a minimal handler for testing compliance logic.
type mockHandler struct {
	enabled      bool
	handleErr    error
	withAttrsNil bool
	withGroupNil bool
}

var (
	_ handler.Handler = (*mockHandler)(nil)
	_ handler.Chainer = (*mockHandler)(nil)
)

func (m *mockHandler) Enabled(level handler.LogLevel) bool {
	return m.enabled
}

func (m *mockHandler) Handle(ctx context.Context, r *handler.Record) error {
	return m.handleErr
}

func (m *mockHandler) HandlerState() handler.HandlerState {
	return nil
}

func (m *mockHandler) WithAttrs(attrs []handler.Attr) handler.Chainer {
	if m.withAttrsNil {
		return nil
	}
	return m
}

func (m *mockHandler) WithGroup(name string) handler.Chainer {
	if m.withGroupNil {
		return nil
	}
	return m
}

// TestNewComplianceChecker verifies the constructor returns a non-nil checker.
func TestNewComplianceChecker(t *testing.T) {
	t.Parallel()
	checker := handler.NewComplianceChecker()
	if checker == nil {
		t.Fatal("NewComplianceChecker() returned nil")
	}
}

// TestComplianceChecker_CheckEnabled verifies enabled state checking.
func TestComplianceChecker_CheckEnabled(t *testing.T) {
	t.Parallel()
	checker := handler.NewComplianceChecker()

	tests := []struct {
		name    string
		enabled bool
		wantErr bool
	}{
		{
			name:    "enabled at info level",
			enabled: true,
			wantErr: false,
		},
		{
			name:    "not enabled at info level",
			enabled: false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := &mockHandler{enabled: tt.enabled}
			err := checker.CheckEnabled(h)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckEnabled() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
		})
	}
}

// TestComplianceChecker_CheckHandle verifies record processing.
func TestComplianceChecker_CheckHandle(t *testing.T) {
	t.Parallel()
	checker := handler.NewComplianceChecker()

	tests := []struct {
		name      string
		handleErr error
		wantErr   bool
	}{
		{
			name:      "successful handle",
			handleErr: nil,
			wantErr:   false,
		},
		{
			name:      "handle returns error",
			handleErr: errors.New("handle failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := &mockHandler{handleErr: tt.handleErr}
			r := &handler.Record{
				Level:   handler.InfoLevel,
				Message: "test message",
				Attrs:   []handler.Attr{{Key: "key", Value: "value"}},
			}

			err := checker.CheckHandle(h, r)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckHandle() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && !errors.Is(err, tt.handleErr) {
				t.Errorf("CheckHandle() error = %v, want %v", err, tt.handleErr)
			}
		})
	}
}

// TestComplianceChecker_CheckChainer verifies chainer interface compliance.
func TestComplianceChecker_CheckChainer(t *testing.T) {
	t.Parallel()
	checker := handler.NewComplianceChecker()

	tests := []struct {
		name         string
		withAttrsNil bool
		withGroupNil bool
		wantErr      bool
		errContains  string
	}{
		{
			name:         "valid chainer",
			withAttrsNil: false,
			withGroupNil: false,
			wantErr:      false,
		},
		{
			name:         "with attrs returns nil",
			withAttrsNil: true,
			withGroupNil: false,
			wantErr:      true,
			errContains:  "WithAttrs",
		},
		{
			name:         "with group returns nil",
			withAttrsNil: false,
			withGroupNil: true,
			wantErr:      true,
			errContains:  "WithGroup",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := &mockHandler{
				withAttrsNil: tt.withAttrsNil,
				withGroupNil: tt.withGroupNil,
			}

			err := checker.CheckChainer(h)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckChainer() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("CheckChainer() error = %q, want to contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}
