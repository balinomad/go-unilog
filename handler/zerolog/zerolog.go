package zerolog

import (
	"context"
	"io"
	"os"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/balinomad/go-atomicwriter"
	"github.com/balinomad/go-unilog"
)

// zerologLogger is a wrapper around Zerolog's logger.
type zerologLogger struct {
	l          zerolog.Logger
	out        *atomicwriter.AtomicWriter
	lvl        atomic.Int32
	withCaller bool
	withTrace  bool
	callerSkip int // Number of stack frames to skip, including internalSkipFrames
}

// Ensure zerologLogger implements the following interfaces.
var (
	_ unilog.Logger        = (*zerologLogger)(nil)
	_ unilog.Configurator  = (*zerologLogger)(nil)
	_ unilog.Cloner        = (*zerologLogger)(nil)
	_ unilog.CallerSkipper = (*zerologLogger)(nil)
)

// New creates a new unilog.Logger instance backed by zerolog.
func New(opts ...ZerologOption) (unilog.Logger, error) {
	o := &zerologOptions{
		level:  unilog.InfoLevel,
		output: os.Stderr,
		format: "json",
	}

	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, unilog.ErrFailedOption(err)
		}
	}

	aw, err := atomicwriter.NewAtomicWriter(o.output)
	if err != nil {
		return nil, unilog.ErrAtomicWriterFail(err)
	}

	var writer io.Writer = aw
	if o.format == "console" {
		writer = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = aw
			w.TimeFormat = time.RFC3339
		})
	}

	// Use TraceLevel for the backend and filter levels within the adapter.
	// This allows for dynamic level changes without re-initializing the logger.
	zl := zerolog.New(writer).Level(zerolog.TraceLevel).With().Timestamp().Logger()

	logger := &zerologLogger{
		l:          zl,
		out:        aw,
		withTrace:  o.withTrace,
		withCaller: o.withCaller,
		callerSkip: o.callerSkip,
	}
	logger.lvl.Store(int32(o.level))

	return logger, nil
}

func (l *zerologLogger) log(level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	zlevel := toZerologLevel(level)
	event := l.l.WithLevel(zlevel)
	if event == nil {
		return
	}

	if l.withCaller {
		event = event.Caller(l.callerSkip + skip + 2)
	}

	if l.withTrace && level >= unilog.ErrorLevel {
		event = event.Str("stack", string(debug.Stack()))
	}

	if len(keyValues) >= 2 {
		event = event.Fields(keyValues)
	}
	event.Msg(msg)

	// Handle termination levels
	switch level {
	case unilog.FatalLevel:
		os.Exit(1)
	case unilog.PanicLevel:
		panic(msg)
	}
}

// Log implements the unilog.Logger interface for zerolog.
func (l *zerologLogger) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(level, msg, 0, keyValues...)
}

// LogWithSkip implements the unilog.Logger interface for zerolog.
func (l *zerologLogger) LogWithSkip(_ context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	l.log(level, msg, skip, keyValues...)
}

// Enabled checks if the given log level is enabled.
func (l *zerologLogger) Enabled(level unilog.LogLevel) bool {
	return level >= unilog.LogLevel(l.lvl.Load())
}

// With returns a new logger with the provided keyValues added to the context.
func (l *zerologLogger) With(keyValues ...any) unilog.Logger {
	if len(keyValues) < 2 {
		return l
	}

	clone := l.clone()
	clone.l = l.l.With().Fields(keyValues).Logger()

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (l *zerologLogger) WithGroup(name string) unilog.Logger {
	if name == "" {
		return l
	}

	clone := l.clone()
	clone.l = l.l.With().Dict(name, zerolog.Dict()).Logger()

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (l *zerologLogger) SetLevel(level unilog.LogLevel) error {
	if err := unilog.ValidateLogLevel(level); err != nil {
		return err
	}
	l.lvl.Store(int32(level))
	return nil
}

// SetOutput sets the log destination.
func (l *zerologLogger) SetOutput(w io.Writer) error {
	return l.out.Swap(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (l *zerologLogger) CallerSkip() int {
	return l.callerSkip
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
func (l *zerologLogger) WithCallerSkip(skip int) (unilog.Logger, error) {
	if skip < 0 {
		return l, unilog.ErrInvalidSourceSkip
	}

	clone := l.clone()
	clone.callerSkip = skip

	return clone, nil
}

// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
func (l *zerologLogger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}
	return l.WithCallerSkip(l.callerSkip + delta)
}

// clone returns a deep copy of the logger.
func (l *zerologLogger) clone() *zerologLogger {
	clone := &zerologLogger{
		l:          l.l,
		out:        l.out,
		withTrace:  l.withTrace,
		callerSkip: l.callerSkip,
	}
	clone.lvl.Store(l.lvl.Load())
	return clone
}

// Clone returns a deep copy of the logger as a unilog.Logger.
func (l *zerologLogger) Clone() unilog.Logger {
	return l.clone()
}

// Trace logs a message at the trace level.
func (l *zerologLogger) Trace(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.TraceLevel, msg, 0, keyValues...)
}

// Debug logs a message at the debug level.
func (l *zerologLogger) Debug(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.DebugLevel, msg, 0, keyValues...)
}

// Info logs a message at the info level.
func (l *zerologLogger) Info(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.InfoLevel, msg, 0, keyValues...)
}

// Warn logs a message at the warn level.
func (l *zerologLogger) Warn(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.WarnLevel, msg, 0, keyValues...)
}

// Error logs a message at the error level.
func (l *zerologLogger) Error(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.ErrorLevel, msg, 0, keyValues...)
}

// Critical logs a message at the critical level.
func (l *zerologLogger) Critical(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.CriticalLevel, msg, 0, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *zerologLogger) Fatal(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.FatalLevel, msg, 0, keyValues...)
}

// Panic logs a message at the panic level and panics.
func (l *zerologLogger) Panic(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.PanicLevel, msg, 0, keyValues...)
}

// toZerologLevel converts a unilog.LogLevel to a zerolog.Level.
func toZerologLevel(level unilog.LogLevel) zerolog.Level {
	level = min(max(level, unilog.MinLevel), unilog.MaxLevel)

	switch level {
	case unilog.TraceLevel:
		return zerolog.TraceLevel
	case unilog.DebugLevel:
		return zerolog.DebugLevel
	case unilog.InfoLevel:
		return zerolog.InfoLevel
	case unilog.WarnLevel:
		return zerolog.WarnLevel
	case unilog.ErrorLevel:
		return zerolog.ErrorLevel
	case unilog.CriticalLevel:
		// Zerolog doesn't have a Critical level, mapping to Error
		return zerolog.ErrorLevel
	case unilog.FatalLevel:
		return zerolog.FatalLevel
	case unilog.PanicLevel:
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}
