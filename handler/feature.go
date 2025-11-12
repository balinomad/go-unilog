package handler

import (
	"fmt"
	"sort"
	"strings"
)

// HandlerFeatures represents a bitmask of features supported by a handler.
type HandlerFeatures struct {
	features Feature
}

// Feature represents a specific feature supported by a handler.
type Feature uint32

const (
	// --- Caller reporting ---

	// Uses backend's caller support (needs caller skip)
	FeatNativeCaller Feature = 1 << iota

	// --- Group/prefix support ---

	// Uses backend's group support (ignores key prefix)
	FeatNativeGroup

	// --- Output characteristics ---

	// Buffers output (implements Syncer)
	FeatBufferedOutput

	// --- Context handling ---

	// Passes context to backend (uses ctx in Handle)
	FeatContextPropagation

	// Supports SetLevel (implements Configurator)
	FeatDynamicLevel

	// Supports SetOutput (implements Configurator)
	FeatDynamicOutput

	// --- Performance characteristics ---

	// Backend designed for zero-allocation logging
	FeatZeroAlloc
)

// NewHandlerFeatures creates a new instance of HandlerFeatures.
func NewHandlerFeatures(features Feature) HandlerFeatures {
	return HandlerFeatures{features: features}
}

// Supports returns true when all provided feature bits are set.
func (hf HandlerFeatures) Supports(mask Feature) bool {
	return hf.features&mask == mask
}

// String returns a human-friendly, stable representation suitable for logging.
//
// Example outputs:
//
//	"none"                             - when no features set
//	"FeatBufferedOutput"               - single feature
//	"FeatBufferedOutput,FeatZeroAlloc" - multiple features sorted by name
//
// Unknown/combined bits are rendered as "0x<hex>".
func (hf HandlerFeatures) String() string {
	if hf.features == 0 {
		return "none"
	}

	// Gather names for individual set bits
	var names []string
	remaining := hf.features

	for bit := Feature(1); bit != 0; bit <<= 1 {
		if remaining&bit == bit {
			names = append(names, bit.String())
			remaining &^= bit
		}
	}

	// if there are any unknown bits left, render as hex
	if remaining != 0 {
		names = append(names, fmt.Sprintf("0x%X", uint32(remaining)))
	}

	// Sort names for stable output
	sort.Strings(names)

	return strings.Join(names, "|")
}

// String returns a stable name for a single Feature bit.
// If the feature value does not match a known single-bit feature it returns "0x<hex>".
func (f Feature) String() string {
	switch f {
	case FeatNativeCaller:
		return "FeatNativeCaller"
	case FeatNativeGroup:
		return "FeatNativeGroup"
	case FeatBufferedOutput:
		return "FeatBufferedOutput"
	case FeatContextPropagation:
		return "FeatContextPropagation"
	case FeatDynamicLevel:
		return "FeatDynamicLevel"
	case FeatDynamicOutput:
		return "FeatDynamicOutput"
	case FeatZeroAlloc:
		return "FeatZeroAlloc"
	default:
		return fmt.Sprintf("0x%X", uint32(f))
	}
}
