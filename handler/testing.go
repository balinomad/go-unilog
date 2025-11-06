package handler

import (
	"context"
	"testing"
)

// ComplianceTest runs basic compliance tests against a Handler implementation.
// Third-party handler authors can use this to verify their implementations.
//
// Example:
//
//	// Static assertions in the base code
//	var _ handler.Handler = (*MyHandler)(nil)
//	var _ handler.Chainer = (*MyHandler)(nil)
//
//	// Compliance tests in the test code
//	func TestMyHandlerCompliance(t *testing.T) {
//	    handler.ComplianceTest(t, func() (handler.Handler, error) {
//	        return newMyHandler(...)
//	    })
//	}
func ComplianceTest(t *testing.T, newHandler func() (Handler, error)) {
	t.Run("Enabled", func(t *testing.T) {
		h, err := newHandler()
		if err != nil {
			t.Fatalf("newHandler() failed: %v", err)
		}

		if !h.Enabled(InfoLevel) {
			t.Error("expected handler to be enabled at InfoLevel")
		}
	})

	t.Run("Handle", func(t *testing.T) {
		h, err := newHandler()
		if err != nil {
			t.Fatalf("newHandler() failed: %v", err)
		}

		r := &Record{
			Level:   InfoLevel,
			Message: "test",
			Attrs:   []Attr{{Key: "key", Value: "value"}},
		}

		if err := h.Handle(context.Background(), r); err != nil {
			t.Errorf("Handle() failed: %v", err)
		}
	})

	t.Run("Chainer", func(t *testing.T) {
		h, err := newHandler()
		if err != nil {
			t.Fatalf("newHandler() failed: %v", err)
		}

		chainer, ok := h.(Chainer)
		if !ok {
			t.Skip("handler does not implement Chainer")
		}

		h2 := chainer.WithAttrs([]Attr{{Key: "test", Value: 1}})
		if h2 == nil {
			t.Error("WithAttrs returned nil")
		}

		h3 := chainer.WithGroup("group")
		if h3 == nil {
			t.Error("WithGroup returned nil")
		}
	})
}
