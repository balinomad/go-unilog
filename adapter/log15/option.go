package log15

import (
	"fmt"
	"io"
	"slices"

	"github.com/balinomad/go-unilog"
)

// Log15Option configures slog logger creation.
type Log15Option func(*log15Options) error

// log15Options holds configuration for log15 logger.
type log15Options struct {
	level      unilog.LogLevel
	output     io.Writer
	format     string // "json" or "term"
	separator  string
	withCaller bool
	withTrace  bool
	callerSkip int
}

// ValidFormats is the list of supported output formats.
var ValidFormats = []string{"json", "term"}

// WithLevel sets the minimum log level.
func WithLevel(level unilog.LogLevel) Log15Option {
	return func(o *log15Options) error {
		if err := unilog.ValidateLogLevel(level); err != nil {
			return fmt.Errorf("WithLevel: %w", err)
		}
		o.level = level
		return nil
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) Log15Option {
	return func(o *log15Options) error {
		if w == nil {
			return fmt.Errorf("WithOutput: %w", unilog.ErrNilWriter)
		}
		o.output = w
		return nil
	}
}

// WithFormat sets the log output format ("json" or "term" for terminal).
func WithFormat(format string) Log15Option {
	return func(o *log15Options) error {
		if !slices.Contains(ValidFormats, format) {
			return fmt.Errorf("WithFormat: %w", unilog.ErrInvalidFormat(format, ValidFormats...))
		}
		o.format = format
		return nil
	}
}

// WithSeparator sets the separator for group key prefixes.
func WithSeparator(separator string) Log15Option {
	return func(o *log15Options) error {
		o.separator = separator
		return nil
	}
}

// WithCaller enables source injection. Optional skip overrides the user skip frames.
func WithCaller(enabled bool, skip ...int) Log15Option {
	return func(o *log15Options) error {
		o.withCaller = enabled
		o.callerSkip = 0
		if enabled && len(skip) > 0 && skip[0] > 0 {
			o.callerSkip = skip[0]
		}
		return nil
	}
}

// WithTrace enables stack traces for ERROR and above.
func WithStackTrace(enabled bool) Log15Option {
	return func(o *log15Options) error {
		o.withTrace = enabled
		return nil
	}
}
