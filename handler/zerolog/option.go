package zerolog

import (
	"fmt"
	"io"
	"slices"

	"github.com/balinomad/go-unilog"
)

// ZerologOption configures the zerolog logger creation.
// ZerologOption configures the Zerolog logger.
type ZerologOption func(*zerologOptions) error

// zerologOptions holds configuration for the Zerolog logger.
type zerologOptions struct {
	level      unilog.LogLevel
	output     io.Writer
	format     string // "json" or "console"
	separator  string
	withCaller bool
	withTrace  bool
	callerSkip int
}

// ValidFormats is the list of supported output formats.
var ValidFormats = []string{"json", "console"}

// WithLevel sets the minimum log level.
func WithLevel(level unilog.LogLevel) ZerologOption {
	return func(o *zerologOptions) error {
		if err := unilog.ValidateLogLevel(level); err != nil {
			return fmt.Errorf("WithLevel: %w", err)
		}
		o.level = level
		return nil
	}
}

// WithOutput sets the log output writer.
func WithOutput(w io.Writer) ZerologOption {
	return func(o *zerologOptions) error {
		if w == nil {
			return fmt.Errorf("WithOutput: %w", unilog.ErrNilWriter)
		}
		o.output = w
		return nil
	}
}

// WithFormat sets the log format ("json" or "console").
func WithFormat(format string) ZerologOption {
	return func(o *zerologOptions) error {
		if !slices.Contains(ValidFormats, format) {
			return fmt.Errorf("WithFormat: %w", unilog.ErrInvalidFormat(format, ValidFormats...))
		}
		o.format = format
		return nil
	}
}

// WithSeparator sets the separator for group key prefixes.
func WithSeparator(separator string) ZerologOption {
	return func(o *zerologOptions) error {
		o.separator = separator
		return nil
	}
}

// WithCaller enables source injection. Optional skip overrides the user skip frames.
func WithCaller(enabled bool, skip ...int) ZerologOption {
	return func(o *zerologOptions) error {
		o.withCaller = enabled
		o.callerSkip = 0
		if enabled && len(skip) > 0 && skip[0] > 0 {
			o.callerSkip = skip[0]
		}
		return nil
	}
}

// WithTrace enables stack trace logging for error-level and above.
func WithTrace(enabled bool) ZerologOption {
	return func(o *zerologOptions) error {
		o.withTrace = enabled
		return nil
	}
}
