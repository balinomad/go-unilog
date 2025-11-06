package handler_test

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

func TestCallerPC(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		skip       int
		expectZero bool
	}{
		{
			name:       "small skip returns non-zero",
			skip:       0,
			expectZero: false,
		},
		{
			name:       "large skip returns zero",
			skip:       1000,
			expectZero: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pc := handler.CallerPC(tc.skip)
			if tc.expectZero {
				if pc != 0 {
					t.Fatalf("CallerPC(%d) = %v; want 0", tc.skip, pc)
				}
				return
			}
			if pc == 0 {
				t.Fatalf("CallerPC(%d) returned 0; expected non-zero PC", tc.skip)
			}
		})
	}
}

func TestPCToLocation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pcGetter func() uintptr
		exp      string
	}{
		{
			name: "pc == 0 returns empty string",
			pcGetter: func() uintptr {
				return 0
			},
			exp: "",
		},
		{
			name: "valid PC returns matching file:line",
			pcGetter: func() uintptr {
				// Capture a PC from this calling site.
				var pcs [1]uintptr
				// runtime.Callers(0, pcs[:]) makes pcs[0] point to this test function (the caller).
				n := runtime.Callers(0, pcs[:])
				if n == 0 {
					return 0
				}
				return pcs[0]
			},
			// exp will be computed inside test because file paths and line numbers
			// are runtime-determined. Put placeholder; test will recompute expected.
			exp: "DYNAMIC",
		},
		{
			name: "non-zero but invalid PC yields \":0\"",
			pcGetter: func() uintptr {
				// Choose an arbitrary small non-zero PC that is unlikely to be valid.
				// Using 1 is a portable choice to exercise runtime.CallersFrames behavior.
				return uintptr(1)
			},
			exp: ":0",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pc := tc.pcGetter()
			got := handler.PCToLocation(pc)

			if tc.exp != "DYNAMIC" {
				if got != tc.exp {
					t.Fatalf("PCToLocation(%v) = %q; want %q", pc, got, tc.exp)
				}
				return
			}

			// Compute expected value for the dynamic case using the same runtime API
			// the implementation uses. This ensures the test does not hardcode file
			// paths or line numbers.
			frames := runtime.CallersFrames([]uintptr{pc})
			frame, _ := frames.Next()
			expected := frame.File + ":" + strconv.Itoa(frame.Line)

			if got != expected {
				t.Fatalf("PCToLocation(%v) = %q; want %q", pc, got, expected)
			}
		})
	}
}
