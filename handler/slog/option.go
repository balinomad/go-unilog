package slog

import (
	"fmt"
	"io"
	"slices"

	"github.com/balinomad/go-unilog"
)

// SlogOption configures slog logger creation.
type SlogOption func(*slogOptions) error

// slogOptions holds configuration for slog logger.
type slogOptions struct {
	level      unilog.LogLevel
	output     io.Writer
	format     string
	withCaller bool
	withTrace  bool
	callerSkip int
}

// ValidFormats is the list of supported output formats.
var ValidFormats = []string{"json", "text"}

// WithLevel sets the minimum log level.
func WithLevel(level unilog.LogLevel) SlogOption {
	return func(o *slogOptions) error {
		if err := unilog.ValidateLogLevel(level); err != nil {
			return fmt.Errorf("WithLevel: %w", err)
		}
		o.level = level
		return nil
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) SlogOption {
	return func(o *slogOptions) error {
		if w == nil {
			return fmt.Errorf("WithOutput: %w", unilog.ErrNilWriter)
		}
		o.output = w
		return nil
	}
}

// WithFormat sets the output format ("json" or "text").
func WithFormat(format string) SlogOption {
	return func(o *slogOptions) error {
		if !slices.Contains(ValidFormats, format) {
			return fmt.Errorf("WithFormat: %w", unilog.ErrInvalidFormat(format, ValidFormats...))
		}
		o.format = format
		return nil
	}
}

// WithCaller enables source injection. Optional skip overrides the user skip frames.
func WithCaller(enabled bool, skip ...int) SlogOption {
	return func(o *slogOptions) error {
		o.withCaller = enabled
		o.callerSkip = 0
		if enabled && len(skip) > 0 && skip[0] > 0 {
			o.callerSkip = skip[0]
		}
		return nil
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) SlogOption {
	return func(o *slogOptions) error {
		o.withTrace = enabled
		return nil
	}
}
