package slog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"sync/atomic"

	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-unilog"
)

// internalSkipFrames is the number of frames to skip within this adapter.
const internalSkipFrames = 3

// slogLogger is a wrapper around Go's standard slog.Logger.
type slogLogger struct {
	l          *slog.Logger
	out        *unilog.AtomicWriter
	lvl        *slog.LevelVar
	withTrace  bool
	withCaller bool
	skipCaller atomic.Int32
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
		level:  unilog.LevelInfo,
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

	aw, err := unilog.NewAtomicWriter(o.output)
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
	}
	l.skipCaller.Store(int32(o.skipCaller + internalSkipFrames))

	return l, nil
}

// log is the internal logging function used by the unilog.Logger interface. It adds caller and
// stack trace information before passing the record to the underlying slog logger.
func (l *slogLogger) log(ctx context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	args := make([]any, 0, len(keyValues)+4)
	args = append(args, keyValues...)

	if l.withCaller {
		skip := int(l.skipCaller.Load())
		args = append(args, slog.SourceKey, caller.New(skip).Location())
	}

	if l.withTrace && level >= unilog.LevelError {
		if len(args)%2 == 1 {
			args = args[:len(args)-1]
		}
		args = append(args, "stack", string(debug.Stack()))
	}

	l.l.Log(ctx, toSlogLevel(level), msg, args...)

	if level == unilog.LevelFatal {
		os.Exit(1)
	}
}

// Log implements the unilog.Logger interface for slog.
func (l *slogLogger) Log(ctx context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(ctx, level, msg, keyValues...)
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
	return int(l.skipCaller.Load() - internalSkipFrames)
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
// The original logger is not modified.
func (l *slogLogger) WithCallerSkip(skip int) (unilog.Logger, error) {
	if skip < 0 {
		return l, unilog.ErrInvalidSourceSkip
	}
	if skip == l.CallerSkip() {
		return l, nil
	}

	clone := l.clone()
	clone.skipCaller.Store(int32(skip + internalSkipFrames))

	return clone, nil
}

// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
// The original logger is not modified.
func (l *slogLogger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}
	return l.WithCallerSkip(l.CallerSkip() + delta)
}

// clone returns a deep copy of the logger.
func (l *slogLogger) clone() *slogLogger {
	clone := &slogLogger{
		l:          l.l,
		out:        l.out,
		lvl:        l.lvl,
		withTrace:  l.withTrace,
		withCaller: l.withCaller,
	}
	clone.skipCaller.Store(l.skipCaller.Load())

	return clone
}

// Clone returns a deep copy of the logger as a unilog.Logger.
func (l *slogLogger) Clone() unilog.Logger {
	return l.clone()
}

// Debug logs a message at the debug level.
func (l *slogLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.LevelDebug, msg, keyValues...)
}

// Info logs a message at the info level.
func (l *slogLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.LevelInfo, msg, keyValues...)
}

// Warn logs a message at the warn level.
func (l *slogLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.LevelWarn, msg, keyValues...)
}

// Error logs a message at the error level.
func (l *slogLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.LevelError, msg, keyValues...)
}

// Critical logs a message at the critical level.
func (l *slogLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.LevelCritical, msg, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *slogLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.LevelFatal, msg, keyValues...)
}

func toSlogLevel(level unilog.LogLevel) slog.Level {
	switch level {
	case unilog.LevelDebug:
		return slog.LevelDebug
	case unilog.LevelInfo:
		return slog.LevelInfo
	case unilog.LevelWarn:
		return slog.LevelWarn
	case unilog.LevelError:
		return slog.LevelError
	case unilog.LevelCritical:
		return slog.Level(10)
	case unilog.LevelFatal:
		return slog.Level(12)
	default:
		return slog.LevelInfo
	}
}
