package multi

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

// mockWriteCloser is a helper for testing that implements io.WriteCloser.
type mockWriteCloser struct {
	buffer     bytes.Buffer
	closeErr   error
	writeErr   error
	closeCalls int
}

func (m *mockWriteCloser) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.buffer.Write(p)
}

func (m *mockWriteCloser) Close() error {
	m.closeCalls++
	return m.closeErr
}

func (m *mockWriteCloser) String() string {
	return m.buffer.String()
}

func TestNew(t *testing.T) {
	t.Run("creates with writers", func(t *testing.T) {
		w1 := &bytes.Buffer{}
		w2 := &bytes.Buffer{}
		mw := New(w1, w2)

		if len(mw.writers) != 2 {
			t.Errorf("expected 2 writers, got %d", len(mw.writers))
		}
	})

	t.Run("creates with no writers", func(t *testing.T) {
		mw := New()
		if len(mw.writers) != 0 {
			t.Errorf("expected 0 writers, got %d", len(mw.writers))
		}
	})
}

func TestMultiWriter_Write(t *testing.T) {
	w1 := &bytes.Buffer{}
	w2 := &mockWriteCloser{}
	mw := New(w1, w2)

	msg := "hello world"
	n, err := mw.Write([]byte(msg))
	if err != nil {
		t.Fatalf("unexpected error during write: %v", err)
	}
	if n != len(msg) {
		t.Errorf("expected to write %d bytes, but wrote %d", len(msg), n)
	}

	if w1.String() != msg {
		t.Errorf("writer 1 content mismatch: got %q, want %q", w1.String(), msg)
	}
	if w2.String() != msg {
		t.Errorf("writer 2 content mismatch: got %q, want %q", w2.String(), msg)
	}
}

func TestMultiWriter_WriteError(t *testing.T) {
	w1 := &bytes.Buffer{}
	expectedErr := errors.New("write failed")
	w2 := &mockWriteCloser{writeErr: expectedErr}
	mw := New(w1, w2)

	msg := "this will fail"
	_, err := mw.Write([]byte(msg))

	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	// The first writer should still have received the content before the error.
	if w1.String() != msg {
		t.Errorf("expected writer 1 to have content %q, got %q", msg, w1.String())
	}
}

func TestMultiWriter_Close(t *testing.T) {
	t.Run("closes underlying closers", func(t *testing.T) {
		w1 := &mockWriteCloser{}
		w2 := &bytes.Buffer{} // Not a closer
		w3 := &mockWriteCloser{}
		mw := New(w1, w2, w3)

		err := mw.Close()
		if err != nil {
			t.Fatalf("unexpected error on close: %v", err)
		}

		if w1.closeCalls != 1 {
			t.Errorf("expected writer 1 to be closed once, got %d", w1.closeCalls)
		}
		if w3.closeCalls != 1 {
			t.Errorf("expected writer 3 to be closed once, got %d", w3.closeCalls)
		}
	})

	t.Run("returns first close error", func(t *testing.T) {
		err1 := errors.New("first error")
		err2 := errors.New("second error")
		w1 := &mockWriteCloser{closeErr: err1}
		w2 := &mockWriteCloser{closeErr: err2}
		mw := New(w1, w2)

		err := mw.Close()
		if !errors.Is(err, err1) {
			t.Errorf("expected error %v, got %v", err1, err)
		}

		// Ensure both are still called
		if w1.closeCalls != 1 || w2.closeCalls != 1 {
			t.Error("expected all closers to be called even if one fails")
		}
	})
}

func TestMultiWriter_AddWriters(t *testing.T) {
	mw := New()
	w1 := &bytes.Buffer{}
	mw.AddWriters(w1)

	if len(mw.writers) != 1 {
		t.Fatalf("expected 1 writer after add, got %d", len(mw.writers))
	}

	msg := "test"
	_, err := mw.Write([]byte(msg))
	if err != nil {
		t.Fatal(err)
	}

	if w1.String() != msg {
		t.Errorf("content mismatch: got %q, want %q", w1.String(), msg)
	}
}

func TestMultiWriter_Concurrency(t *testing.T) {
	var wg sync.WaitGroup
	w1 := &bytes.Buffer{}
	w2 := &bytes.Buffer{}
	mw := New(w1, w2)

	numGoroutines := 100
	writesPerGoroutine := 10
	msg := "concurrent write\n"

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				_, err := mw.Write([]byte(msg))
				if err != nil {
					t.Errorf("concurrent write failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	expectedSize := numGoroutines * writesPerGoroutine * len(msg)
	if w1.Len() != expectedSize {
		t.Errorf("writer 1 has incorrect size: got %d, want %d", w1.Len(), expectedSize)
	}
	if w2.Len() != expectedSize {
		t.Errorf("writer 2 has incorrect size: got %d, want %d", w2.Len(), expectedSize)
	}
}
