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

// Feature represents backend implementation characteristics, NOT API contracts.
// Use interface type assertions for API detection.
//
// Features answer: "How does the backend work internally?"
// Interfaces answer: "What API methods are available?"
//
// Example:
//   - FeatNativeCaller: backend accepts skip parameter (zap.AddCallerSkip)
//   - AdvancedHandler interface: exposes WithCallerSkip() method
//
// A handler can implement AdvancedHandler (API) without FeatNativeCaller (backend).
// In this case, WithCallerSkip() would be emulated using runtime.Caller().
type Feature uint32

const (
	// --- Caller reporting ---

	// FeatNativeCaller: Backend supports native caller skip (e.g., zap.AddCallerSkip).
	// If false, unilog captures PC via runtime.Caller and passes to Record.PC.
	FeatNativeCaller Feature = 1 << iota

	// --- Group/prefix support ---

	// FeatNativeGroup: Backend supports native grouping (e.g., zap.Namespace, slog.WithGroup).
	// If false, handler must manually prefix keys using BaseHandler.keyPrefix.
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
