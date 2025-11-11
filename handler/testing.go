package handler

import (
	"context"
	"errors"
	"testing"
)

// ComplianceChecker provides granular compliance verification for handler implementations.
// Most users should use ComplianceTest directly. This interface is primarily for
// custom test scenarios and internal testing of the compliance logic itself.
type ComplianceChecker interface {
	// CheckEnabled verifies that the handler correctly implements the Enabled method.
	// Returns an error if the handler is not enabled at InfoLevel.
	CheckEnabled(h Handler) error

	// CheckHandle verifies that the handler correctly processes log records.
	// Returns an error if Handle returns an error for a valid record.
	CheckHandle(h Handler, r *Record) error

	// CheckChainer verifies that the handler correctly implements the Chainer interface.
	// Returns an error if WithAttrs or WithGroup return nil.
	// This method should only be called if the handler implements Chainer.
	CheckChainer(c Chainer) error
}

// NewComplianceChecker returns a new compliance checker instance.
func NewComplianceChecker() ComplianceChecker {
	return &checker{}
}

type checker struct{}

// Ensure checker implements ComplianceChecker
var _ ComplianceChecker = (*checker)(nil)

// CheckEnabled verifies that the handler is enabled at InfoLevel.
func (c *checker) CheckEnabled(h Handler) error {
	if !h.Enabled(InfoLevel) {
		return errors.New("handler not enabled at InfoLevel")
	}
	return nil
}

// CheckHandle verifies that the handler processes a valid record without error.
func (c *checker) CheckHandle(h Handler, r *Record) error {
	if err := h.Handle(context.Background(), r); err != nil {
		return err
	}
	return nil
}

// CheckChainer verifies that Chainer methods return non-nil handlers.
func (c *checker) CheckChainer(ch Chainer) error {
	h := ch.WithAttrs([]any{"test", 1})
	if h == nil {
		return errors.New("WithAttrs returned nil")
	}

	h = ch.WithGroup("group")
	if h == nil {
		return errors.New("WithGroup returned nil")
	}

	return nil
}

// ComplianceTest runs comprehensive compliance tests against a Handler implementation.
// Third-party handler authors can use this to verify their implementations meet
// the handler.Handler interface contract.
//
// The test suite covers:
//   - Enabled: Verifies handler is enabled at InfoLevel
//   - Handle: Verifies handler processes valid records without error
//   - Chainer: Verifies Chainer methods return non-nil (skipped if not implemented)
//
// Example usage:
//
//	// Optional static assertions
//	var _ handler.Handler = (*MyHandler)(nil)
//	var _ handler.Chainer = (*MyHandler)(nil) // If Chainer is implemented
//
//	// Compliance tests in the test code
//	func TestMyHandlerCompliance(t *testing.T) {
//	    handler.ComplianceTest(t, func() (handler.Handler, error) {
//	        return NewMyHandler(...)
//	    })
//	}
func ComplianceTest(t *testing.T, newHandler func() (Handler, error)) {
	t.Helper()
	checker := NewComplianceChecker()

	t.Run("enabled", func(t *testing.T) {
		h, err := newHandler()
		if err != nil {
			t.Fatalf("newHandler() failed: %v", err)
		}

		if err := checker.CheckEnabled(h); err != nil {
			t.Error(err)
		}
	})

	t.Run("handle", func(t *testing.T) {
		h, err := newHandler()
		if err != nil {
			t.Fatalf("newHandler() failed: %v", err)
		}

		r := &Record{
			Level:     InfoLevel,
			Message:   "test",
			KeyValues: []any{"key", "value"},
		}

		if err := checker.CheckHandle(h, r); err != nil {
			t.Errorf("Handle() failed: %v", err)
		}
	})

	t.Run("chainer", func(t *testing.T) {
		h, err := newHandler()
		if err != nil {
			t.Fatalf("newHandler() failed: %v", err)
		}

		chainer, ok := h.(Chainer)
		if !ok {
			t.Skip("handler does not implement Chainer")
			return
		}

		if err := checker.CheckChainer(chainer); err != nil {
			t.Error(err)
		}
	})
}
