package stdlog

import (
	"fmt"
	"io"

	"github.com/balinomad/go-unilog"
)

// LogOption configures the standard logger creation.
type LogOption func(*logOptions) error

// logOptions holds configuration for the standard logger.
type logOptions struct {
	level      unilog.LogLevel
	output     io.Writer
	separator  string
	withCaller bool
	withTrace  bool
	skipCaller int
}

// WithLevel sets the minimum log level.
func WithLevel(level unilog.LogLevel) LogOption {
	return func(o *logOptions) error {
		if err := unilog.ValidateLogLevel(level); err != nil {
			return fmt.Errorf("WithLevel: %w", err)
		}
		o.level = level
		return nil
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) LogOption {
	return func(o *logOptions) error {
		if w == nil {
			return fmt.Errorf("WithOutput: %w", unilog.ErrNilWriter)
		}
		o.output = w
		return nil
	}
}

// WithSeparator sets the separator for group key prefixes.
func WithSeparator(separator string) LogOption {
	return func(o *logOptions) error {
		o.separator = separator
		return nil
	}
}

// WithCaller enables source injection. Optional skip overrides the user skip frames.
func WithCaller(enabled bool, skip ...int) LogOption {
	return func(o *logOptions) error {
		o.withCaller = enabled
		o.skipCaller = 0
		if enabled && len(skip) > 0 && skip[0] > 0 {
			o.skipCaller = skip[0]
		}
		return nil
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithTrace(enabled bool) LogOption {
	return func(o *logOptions) error {
		o.withTrace = enabled
		return nil
	}
}
