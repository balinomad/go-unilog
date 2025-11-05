package logrus

import (
	"fmt"
	"io"
	"slices"

	"github.com/balinomad/go-unilog"
)

// LogrusOption defines a configuration function for the logrus logger.
type LogrusOption func(*logrusOptions) error

// logrusOptions holds configuration options for the logrus logger.
type logrusOptions struct {
	level      unilog.LogLevel
	output     io.Writer
	format     string // "json" or "text"
	separator  string
	withCaller bool
	withTrace  bool
	callerSkip int
}

// ValidFormats is the list of supported output formats.
var ValidFormats = []string{"json", "text"}

// WithLevel sets the minimum log level.
func WithLevel(level unilog.LogLevel) LogrusOption {
	return func(o *logrusOptions) error {
		if err := unilog.ValidateLogLevel(level); err != nil {
			return fmt.Errorf("WithLevel: %w", err)
		}
		o.level = level
		return nil
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) LogrusOption {
	return func(o *logrusOptions) error {
		if w == nil {
			return fmt.Errorf("WithOutput: %w", unilog.ErrNilWriter)
		}
		o.output = w
		return nil
	}
}

// WithFormat sets the output format ("json" or "text").
func WithFormat(format string) LogrusOption {
	return func(o *logrusOptions) error {
		if !slices.Contains(ValidFormats, format) {
			return fmt.Errorf("WithFormat: %w", unilog.ErrInvalidFormat(format, ValidFormats...))
		}
		o.format = format
		return nil
	}
}

// WithSeparator sets the separator for group key prefixes.
func WithSeparator(separator string) LogrusOption {
	return func(o *logrusOptions) error {
		o.separator = separator
		return nil
	}
}

// WithCaller enables source injection. Optional skip overrides the user skip frames.
func WithCaller(enabled bool, skip ...int) LogrusOption {
	return func(o *logrusOptions) error {
		o.withCaller = enabled
		o.callerSkip = 0
		if enabled && len(skip) > 0 && skip[0] > 0 {
			o.callerSkip = skip[0]
		}
		return nil
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(addStackTrace bool) LogrusOption {
	return func(o *logrusOptions) error {
		o.withTrace = addStackTrace
		return nil
	}
}
