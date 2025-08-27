package slog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/balinomad/go-atomicwriter"
	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-unilog"
)

// slogLogger is a wrapper around Go's standard slog.Logger.
type slogLogger struct {
	l          *slog.Logger
	out        *atomicwriter.AtomicWriter
	lvl        *slog.LevelVar
	withTrace  bool
	withCaller bool
	callerSkip int
}

// Ensure slogLogger implements the following interfaces.
var (
	_ unilog.Logger        = (*slogLogger)(nil)
	_ unilog.Configurator  = (*slogLogger)(nil)
	_ unilog.Cloner        = (*slogLogger)(nil)
	_ unilog.CallerSkipper = (*slogLogger)(nil)
)

// New creates a new unilog.Logger instance backed by log/slog.
func New(opts ...SlogOption) (unilog.Logger, error) {
	o := &slogOptions{
		level:  unilog.InfoLevel,
		output: os.Stderr,
		format: "json",
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, unilog.ErrFailedOption(err)
		}
	}

	levelVar := new(slog.LevelVar)
	levelVar.Set(toSlogLevel(o.level))

	aw, err := atomicwriter.NewAtomicWriter(o.output)
	if err != nil {
		return nil, unilog.ErrAtomicWriterFail(err)
	}

	handlerOpts := &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: false, // we set this manually
	}

	var handler slog.Handler
	if o.format == "text" {
		handler = slog.NewTextHandler(aw, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(aw, handlerOpts)
	}

	l := &slogLogger{
		l:          slog.New(handler),
		lvl:        levelVar,
		out:        aw,
		withTrace:  o.withTrace,
		withCaller: o.withCaller,
		callerSkip: o.callerSkip,
	}

	return l, nil
}

// log is the internal logging function used by the unilog.Logger interface. It adds caller and
// stack trace information before passing the record to the underlying slog logger.
func (l *slogLogger) log(ctx context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	args := make([]any, 0, len(keyValues)+4)
	args = append(args, keyValues...)
	if len(args)%2 == 1 {
		args = args[:len(args)-1]
	}

	if l.withCaller {
		// Add 2 to skip this function and the caller function
		if s := l.callerSkip + skip + 2; s > 0 {
			args = append(args, slog.SourceKey, caller.New(s).Location())
		}
	}

	if l.withTrace && level >= unilog.ErrorLevel {
		args = append(args, "stack", string(debug.Stack()))
	}

	l.l.Log(ctx, toSlogLevel(level), msg, args...)

	// Handle termination levels
	switch level {
	case unilog.FatalLevel:
		os.Exit(1)
	case unilog.PanicLevel:
		panic(msg)
	}
}

// Log implements the unilog.Logger interface for slog.
func (l *slogLogger) Log(ctx context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(ctx, level, msg, 0, keyValues...)
}

// LogWithSkip implements the unilog.CallerSkipper interface for slog.
func (l *slogLogger) LogWithSkip(ctx context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	l.log(ctx, level, msg, skip, keyValues...)
}

// Enabled checks if the given log level is enabled.
func (l *slogLogger) Enabled(level unilog.LogLevel) bool {
	return l.l.Enabled(context.Background(), toSlogLevel(level))
}

// With returns a new logger with the provided keyValues added to the context.
func (l *slogLogger) With(keyValues ...any) unilog.Logger {
	if len(keyValues) < 2 {
		return l
	}

	clone := l.clone()
	clone.l = clone.l.With(keyValues...)

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (l *slogLogger) WithGroup(name string) unilog.Logger {
	if name == "" {
		return l
	}

	clone := l.clone()
	clone.l = clone.l.WithGroup(name)

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (l *slogLogger) SetLevel(level unilog.LogLevel) error {
	if err := unilog.ValidateLogLevel(level); err != nil {
		return err
	}

	l.lvl.Set(toSlogLevel(level))

	return nil
}

// SetOutput sets the log destination.
func (l *slogLogger) SetOutput(w io.Writer) error {
	return l.out.Swap(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (l *slogLogger) CallerSkip() int {
	return l.callerSkip
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
func (l *slogLogger) WithCallerSkip(skip int) (unilog.Logger, error) {
	if skip < 0 {
		return l, unilog.ErrInvalidSourceSkip
	}

	if skip == l.callerSkip {
		return l, nil
	}

	clone := l.clone()
	clone.callerSkip = skip

	return clone, nil
}

// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
func (l *slogLogger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}

	return l.WithCallerSkip(l.callerSkip + delta)
}

// clone returns a deep copy of the logger.
func (l *slogLogger) clone() *slogLogger {
	return &slogLogger{
		l:          l.l,
		out:        l.out,
		lvl:        l.lvl,
		withTrace:  l.withTrace,
		withCaller: l.withCaller,
		callerSkip: l.callerSkip,
	}
}

// Clone returns a deep copy of the logger as a unilog.Logger.
func (l *slogLogger) Clone() unilog.Logger {
	return l.clone()
}

// Trace logs a message at the trace level.
func (l *slogLogger) Trace(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.TraceLevel, msg, 0, keyValues...)
}

// Debug logs a message at the debug level.
func (l *slogLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.DebugLevel, msg, 0, keyValues...)
}

// Info logs a message at the info level.
func (l *slogLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.InfoLevel, msg, 0, keyValues...)
}

// Warn logs a message at the warn level.
func (l *slogLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.WarnLevel, msg, 0, keyValues...)
}

// Error logs a message at the error level.
func (l *slogLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.ErrorLevel, msg, 0, keyValues...)
}

// Critical logs a message at the critical level.
func (l *slogLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.CriticalLevel, msg, 0, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *slogLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.FatalLevel, msg, 0, keyValues...)
}

// Panic logs a message at the panic level and panics.
func (l *slogLogger) Panic(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.PanicLevel, msg, 0, keyValues...)
}

func toSlogLevel(level unilog.LogLevel) slog.Level {
	level = min(max(level, unilog.MinLevel), unilog.MaxLevel)

	switch level {
	case unilog.TraceLevel:
		return slog.Level(-8)
	case unilog.DebugLevel:
		return slog.LevelDebug
	case unilog.InfoLevel:
		return slog.LevelInfo
	case unilog.WarnLevel:
		return slog.LevelWarn
	case unilog.ErrorLevel:
		return slog.LevelError
	case unilog.CriticalLevel:
		return slog.Level(12)
	case unilog.FatalLevel:
		return slog.Level(16)
	case unilog.PanicLevel:
		return slog.Level(20)
	default:
		return slog.LevelInfo
	}
}
