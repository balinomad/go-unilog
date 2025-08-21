package zap

import (
	"fmt"
	"io"
	"slices"

	"github.com/balinomad/go-unilog"
)

// ZapOption configures the zap logger creation.
type ZapOption func(*zapOptions) error

// zapOptions holds configuration for the zap logger.
type zapOptions struct {
	level      unilog.LogLevel
	output     io.Writer
	format     string // "json" or "console"
	separator  string
	useSugar   bool
	withCaller bool
	withTrace  bool
	callerSkip int
}

// ValidFormats is the list of supported output formats.
var ValidFormats = []string{"json", "console"}

// WithLevel sets the minimum log level.
func WithLevel(level unilog.LogLevel) ZapOption {
	return func(o *zapOptions) error {
		if err := unilog.ValidateLogLevel(level); err != nil {
			return fmt.Errorf("WithLevel: %w", err)
		}
		o.level = level
		return nil
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) ZapOption {
	return func(o *zapOptions) error {
		if w == nil {
			return fmt.Errorf("WithOutput: %w", unilog.ErrNilWriter)
		}
		o.output = w
		return nil
	}
}

// WithFormat sets the output format ("json" or "console").
func WithFormat(format string) ZapOption {
	return func(o *zapOptions) error {
		if !slices.Contains(ValidFormats, format) {
			return fmt.Errorf("WithFormat: %w", unilog.ErrInvalidFormat(format, ValidFormats...))
		}
		o.format = format
		return nil
	}
}

// WithSeparator sets the separator for group key prefixes.
func WithSeparator(separator string) ZapOption {
	return func(o *zapOptions) error {
		o.separator = separator
		return nil
	}
}

// WithSugar enables the zap sugar logger for more convenient logging.
func WithSugar(useSugar bool) ZapOption {
	return func(o *zapOptions) error {
		o.useSugar = useSugar
		return nil
	}
}

// WithCaller enables source injection. Optional skip overrides the user skip frames.
func WithCaller(enabled bool, skip ...int) ZapOption {
	return func(o *zapOptions) error {
		o.withCaller = enabled
		o.callerSkip = 0
		if enabled && len(skip) > 0 && skip[0] > 0 {
			o.callerSkip = skip[0]
		}
		return nil
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) ZapOption {
	return func(o *zapOptions) error {
		o.withTrace = enabled
		return nil
	}
}
