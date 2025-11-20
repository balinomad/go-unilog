package handler_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

// allKnownFeatures is a helper constant for testing.
// It must be manually updated if new features are added.
const allKnownFeatures = handler.FeatNativeCaller |
	handler.FeatNativeGroup |
	handler.FeatBufferedOutput |
	handler.FeatContextPropagation |
	handler.FeatDynamicLevel |
	handler.FeatDynamicOutput |
	handler.FeatZeroAlloc

// TestHandlerFeatures_Supports verifies the feature-checking logic.
func TestHandlerFeatures_Supports(t *testing.T) {
	t.Parallel()

	// An unknown feature bit for testing
	const featUnknown handler.Feature = 1 << 31

	tests := []struct {
		name string
		hf   handler.HandlerFeatures
		mask handler.Feature
		want bool
	}{
		{
			name: "none check zero",
			hf:   handler.NewHandlerFeatures(0),
			mask: 0,
			want: true,
		},
		{
			name: "none check one",
			hf:   handler.NewHandlerFeatures(0),
			mask: handler.FeatNativeCaller,
			want: false,
		},
		{
			name: "none check two bits",
			hf:   handler.NewHandlerFeatures(0),
			mask: handler.FeatNativeCaller | handler.FeatNativeGroup,
			want: false,
		},
		{
			name: "one check zero",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller),
			mask: 0,
			want: true,
		},
		{
			name: "one check self",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller),
			mask: handler.FeatNativeCaller,
			want: true,
		},
		{
			name: "one check other",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller),
			mask: handler.FeatNativeGroup,
			want: false,
		},
		{
			name: "one check self and other bits",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller),
			mask: handler.FeatNativeCaller | handler.FeatNativeGroup,
			want: false,
		},
		{
			name: "two check one arg a",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller | handler.FeatZeroAlloc),
			mask: handler.FeatNativeCaller,
			want: true,
		},
		{
			name: "two check one arg b",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller | handler.FeatZeroAlloc),
			mask: handler.FeatZeroAlloc,
			want: true,
		},
		{
			name: "two check other",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller | handler.FeatZeroAlloc),
			mask: handler.FeatNativeGroup,
			want: false,
		},
		{
			name: "two check two bits",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller | handler.FeatZeroAlloc),
			mask: handler.FeatNativeCaller | handler.FeatZeroAlloc,
			want: true,
		},
		{
			name: "two check three bits fail",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller | handler.FeatZeroAlloc),
			mask: handler.FeatNativeCaller | handler.FeatZeroAlloc | handler.FeatNativeGroup,
			want: false,
		},
		{
			name: "all check all bits",
			hf:   handler.NewHandlerFeatures(allKnownFeatures),
			mask: allKnownFeatures,
			want: true,
		},
		{
			name: "all check all and unknown bits",
			hf:   handler.NewHandlerFeatures(allKnownFeatures),
			mask: allKnownFeatures | featUnknown,
			want: false,
		},
		{
			name: "all with unknown check unknown",
			hf:   handler.NewHandlerFeatures(allKnownFeatures | featUnknown),
			mask: featUnknown,
			want: true,
		},
		{
			name: "all with unknown check all and unknown",
			hf:   handler.NewHandlerFeatures(allKnownFeatures | featUnknown),
			mask: allKnownFeatures | featUnknown,
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.hf.Supports(tt.mask); got != tt.want {
				t.Errorf("HandlerFeatures.Supports() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHandlerFeatures_String tests the human-readable string output.
func TestHandlerFeatures_String(t *testing.T) {
	t.Parallel()

	// An unknown feature bit for testing
	const featUnknown handler.Feature = 1 << 31
	const featUnknown2 handler.Feature = 1 << 30

	tests := []struct {
		name string
		hf   handler.HandlerFeatures
		want string
	}{
		{
			name: "none",
			hf:   handler.NewHandlerFeatures(0),
			want: "none",
		},
		{
			name: "single",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller),
			want: "FeatNativeCaller",
		},
		{
			name: "single high bit",
			hf:   handler.NewHandlerFeatures(handler.FeatZeroAlloc),
			want: "FeatZeroAlloc",
		},
		{
			name: "two sorted",
			hf:   handler.NewHandlerFeatures(handler.FeatZeroAlloc | handler.FeatNativeCaller),
			want: "FeatNativeCaller|FeatZeroAlloc",
		},
		{
			name: "two unsorted",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller | handler.FeatZeroAlloc),
			want: "FeatNativeCaller|FeatZeroAlloc",
		},
		{
			name: "all",
			hf:   handler.NewHandlerFeatures(allKnownFeatures),
			// Must be lexically sorted
			want: strings.Join([]string{
				"FeatBufferedOutput",
				"FeatContextPropagation",
				"FeatDynamicLevel",
				"FeatDynamicOutput",
				"FeatNativeCaller",
				"FeatNativeGroup",
				"FeatZeroAlloc",
			}, "|"),
		},
		{
			name: "unknown only",
			hf:   handler.NewHandlerFeatures(featUnknown),
			want: fmt.Sprintf("0x%X", uint32(featUnknown)),
		},
		{
			name: "unknown and known",
			hf:   handler.NewHandlerFeatures(handler.FeatNativeCaller | featUnknown),
			want: fmt.Sprintf("0x%X|FeatNativeCaller", uint32(featUnknown)),
		},
		{
			name: "unknown and all",
			hf:   handler.NewHandlerFeatures(allKnownFeatures | featUnknown),
			want: fmt.Sprintf("0x%X|%s", uint32(featUnknown), strings.Join([]string{
				"FeatBufferedOutput",
				"FeatContextPropagation",
				"FeatDynamicLevel",
				"FeatDynamicOutput",
				"FeatNativeCaller",
				"FeatNativeGroup",
				"FeatZeroAlloc",
			}, "|")),
		},
		{
			name: "two unknown",
			hf:   handler.NewHandlerFeatures(featUnknown | featUnknown2),
			want: fmt.Sprintf("0x%X|0x%X", uint32(featUnknown2), uint32(featUnknown)),
		},
		{
			name: "two unknown and known",
			hf:   handler.NewHandlerFeatures(featUnknown | handler.FeatNativeCaller | featUnknown2),
			want: fmt.Sprintf("0x%X|0x%X|FeatNativeCaller", uint32(featUnknown2), uint32(featUnknown)),
		},
		{
			name: "non contiguous unknown bits",
			hf:   handler.NewHandlerFeatures((1 << 29) | (1 << 31)),
			want: fmt.Sprintf("0x%X|0x%X", uint32(1<<29), uint32(1<<31)),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.hf.String(); got != tt.want {
				t.Errorf("HandlerFeatures.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFeature_String tests the string name for individual feature bits.
func TestFeature_String(t *testing.T) {
	t.Parallel()

	// An unknown feature bit for testing
	const featUnknown handler.Feature = 1 << 31

	tests := []struct {
		name string
		f    handler.Feature
		want string
	}{
		{
			name: "native caller",
			f:    handler.FeatNativeCaller,
			want: "FeatNativeCaller",
		},
		{
			name: "native group",
			f:    handler.FeatNativeGroup,
			want: "FeatNativeGroup",
		},
		{
			name: "buffered output",
			f:    handler.FeatBufferedOutput,
			want: "FeatBufferedOutput",
		},
		{
			name: "context propagation",
			f:    handler.FeatContextPropagation,
			want: "FeatContextPropagation",
		},
		{
			name: "dynamic level",
			f:    handler.FeatDynamicLevel,
			want: "FeatDynamicLevel",
		},
		{
			name: "dynamic output",
			f:    handler.FeatDynamicOutput,
			want: "FeatDynamicOutput",
		},
		{
			name: "zero alloc",
			f:    handler.FeatZeroAlloc,
			want: "FeatZeroAlloc",
		},
		{
			name: "zero",
			f:    0,
			want: "0x0",
		},
		{
			name: "unknown",
			f:    featUnknown,
			want: fmt.Sprintf("0x%X", uint32(featUnknown)),
		},
		{
			name: "combined",
			f:    handler.FeatNativeCaller | handler.FeatNativeGroup,
			want: fmt.Sprintf("0x%X", uint32(handler.FeatNativeCaller|handler.FeatNativeGroup)),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.f.String(); got != tt.want {
				t.Errorf("Feature.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
