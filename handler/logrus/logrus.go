package logrus

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/sirupsen/logrus"

	"github.com/balinomad/go-atomicwriter"
	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-unilog"
)

// logrusLogger is a wrapper around Logrus's logger.
type logrusLogger struct {
	entry      *logrus.Entry
	out        *atomicwriter.AtomicWriter
	keyPrefix  string
	separator  string
	withTrace  bool
	withCaller bool
	callerSkip int
}

// Ensure logrusLogger implements the following interfaces.
var (
	_ unilog.Logger        = (*logrusLogger)(nil)
	_ unilog.Configurator  = (*logrusLogger)(nil)
	_ unilog.Cloner        = (*logrusLogger)(nil)
	_ unilog.CallerSkipper = (*logrusLogger)(nil)
)

// New creates a new unilog.Logger instance backed by logrus.
func New(opts ...LogrusOption) (unilog.Logger, error) {
	o := &logrusOptions{
		level:     unilog.InfoLevel,
		output:    os.Stderr,
		format:    "json",
		separator: "_",
	}

	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	aw, err := atomicwriter.NewAtomicWriter(o.output)
	if err != nil {
		return nil, unilog.ErrAtomicWriterFail(err)
	}

	l := logrus.New()
	l.SetOutput(aw)
	l.SetLevel(toLogrusLevel(o.level))
	l.SetReportCaller(false)

	if o.format == "json" {
		l.SetFormatter(&logrus.JSONFormatter{})
	} else {
		l.SetFormatter(&logrus.TextFormatter{DisableQuote: true})
	}

	logger := &logrusLogger{
		entry:      logrus.NewEntry(l),
		out:        aw,
		separator:  o.separator,
		withTrace:  o.withTrace,
		withCaller: o.withCaller,
		callerSkip: o.callerSkip,
	}

	return logger, nil
}

func (l *logrusLogger) log(ctx context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	fields := l.processKeyValues(keyValues...)

	if l.withCaller {
		if s := l.callerSkip + skip + 2; s > 0 {
			fields["source"] = caller.New(s).Location()
		}
	}

	if l.withTrace && level >= unilog.ErrorLevel {
		fields["stack"] = string(debug.Stack())
	}

	entry := l.entry.WithFields(fields)
	if ctx != nil && ctx != context.Background() {
		entry = entry.WithContext(ctx)
	}

	entry.Log(toLogrusLevel(level), msg)

	// Handle termination levels
	switch level {
	case unilog.FatalLevel:
		os.Exit(1)
	case unilog.PanicLevel:
		panic(msg)
	}
}

// Log implements the unilog.Logger interface for logrus.
func (l *logrusLogger) Log(ctx context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(ctx, level, msg, 0, keyValues...)
}

// LogWithSkip implements the unilog.CallerSkipper interface for logrus.
func (l *logrusLogger) LogWithSkip(ctx context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	l.log(ctx, level, msg, skip, keyValues...)
}

// With returns a new logger with the provided keyValues added to the context.
func (l *logrusLogger) With(keyValues ...any) unilog.Logger {
	if len(keyValues) < 2 {
		return l
	}

	clone := l.clone()
	clone.entry = l.entry.WithFields(l.processKeyValues(keyValues...))

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (l *logrusLogger) WithGroup(name string) unilog.Logger {
	if name == "" {
		return l
	}

	clone := l.clone()
	if l.keyPrefix == "" {
		clone.keyPrefix = name
	} else {
		clone.keyPrefix = l.keyPrefix + l.separator + name
	}

	return clone
}

// Enabled checks if the given log level is enabled.
func (l *logrusLogger) Enabled(level unilog.LogLevel) bool {
	return l.entry.Logger.IsLevelEnabled(toLogrusLevel(level))
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (l *logrusLogger) SetLevel(level unilog.LogLevel) error {
	if err := unilog.ValidateLogLevel(level); err != nil {
		return err
	}
	l.entry.Logger.SetLevel(toLogrusLevel(level))

	return nil
}

// SetOutput changes the destination for log output.
func (l *logrusLogger) SetOutput(w io.Writer) error {
	return l.out.Swap(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (l *logrusLogger) CallerSkip() int {
	return l.callerSkip
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
func (l *logrusLogger) WithCallerSkip(skip int) (unilog.Logger, error) {
	if skip < 0 {
		return l, unilog.ErrInvalidSourceSkip
	}

	clone := l.clone()
	clone.callerSkip = skip

	return clone, nil
}

// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
func (l *logrusLogger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}
	return l.WithCallerSkip(l.CallerSkip() + delta)
}

// processKeyValues processes the given keyValues and returns a logrus.Fields object.
func (l *logrusLogger) processKeyValues(keyValues ...any) logrus.Fields {
	fields := make(logrus.Fields, len(keyValues)/2)
	prefix := ""
	if l.keyPrefix != "" {
		prefix = l.keyPrefix + l.separator
	}

	for i := 0; i < len(keyValues)-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprint(keyValues[i])
		}
		fields[prefix+key] = keyValues[i+1]
	}
	return fields
}

// clone returns a deep copy of the logger.
func (l *logrusLogger) clone() *logrusLogger {
	return &logrusLogger{
		entry:      l.entry,
		out:        l.out,
		keyPrefix:  l.keyPrefix,
		separator:  l.separator,
		withTrace:  l.withTrace,
		withCaller: l.withCaller,
		callerSkip: l.callerSkip,
	}
}

// Clone returns a deep copy of the logger as a unilog.Logger.
func (l *logrusLogger) Clone() unilog.Logger {
	return l.clone()
}

// Trace logs a message at the trace level.
func (l *logrusLogger) Trace(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.TraceLevel, msg, 0, keyValues...)
}

// Debug logs a message at the debug level.
func (l *logrusLogger) Debug(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.DebugLevel, msg, 0, keyValues...)
}

// Info logs a message at the info level.
func (l *logrusLogger) Info(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.InfoLevel, msg, 0, keyValues...)
}

// Warn logs a message at the warn level.
func (l *logrusLogger) Warn(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.WarnLevel, msg, 0, keyValues...)
}

// Error logs a message at the error level.
func (l *logrusLogger) Error(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.ErrorLevel, msg, 0, keyValues...)
}

// Critical logs a message at the critical level.
func (l *logrusLogger) Critical(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.CriticalLevel, msg, 0, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *logrusLogger) Fatal(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.FatalLevel, msg, 0, keyValues...)
}

// Panic logs a message at the panic level and panics.
func (l *logrusLogger) Panic(ctx context.Context, msg string, keyValues ...any) {
	l.log(ctx, unilog.PanicLevel, msg, 0, keyValues...)
}

func toLogrusLevel(level unilog.LogLevel) logrus.Level {
	level = min(max(level, unilog.MinLevel), unilog.MaxLevel)

	switch level {
	case unilog.TraceLevel:
		return logrus.TraceLevel
	case unilog.DebugLevel:
		return logrus.DebugLevel
	case unilog.InfoLevel:
		return logrus.InfoLevel
	case unilog.WarnLevel:
		return logrus.WarnLevel
	case unilog.ErrorLevel:
		return logrus.ErrorLevel
	case unilog.CriticalLevel:
		return logrus.ErrorLevel
	case unilog.FatalLevel:
		return logrus.FatalLevel
	case unilog.PanicLevel:
		return logrus.PanicLevel
	default:
		return logrus.InfoLevel
	}
}
