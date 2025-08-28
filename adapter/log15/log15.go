package log15

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync/atomic"

	"github.com/inconshreveable/log15/v3"

	"github.com/balinomad/go-atomicwriter"
	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-unilog"
)

// log15Logger is a wrapper around log15's logger.
type log15Logger struct {
	l          log15.Logger
	out        *atomicwriter.AtomicWriter
	lvl        atomic.Int32
	keyPrefix  string
	separator  string
	withTrace  bool
	withCaller bool
	callerSkip int
}

// Ensure log15Logger implements the following interfaces.
var (
	_ unilog.Logger        = (*log15Logger)(nil)
	_ unilog.Configurator  = (*log15Logger)(nil)
	_ unilog.Cloner        = (*log15Logger)(nil)
	_ unilog.CallerSkipper = (*log15Logger)(nil)
)

// New creates a new unilog.Logger instance backed by log15.
func New(opts ...Log15Option) (unilog.Logger, error) {
	o := &log15Options{
		level:     unilog.InfoLevel,
		output:    os.Stderr,
		format:    "json",
		separator: "_",
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

	var format log15.Format
	if o.format == "json" {
		format = log15.JsonFormat()
	} else {
		format = log15.TerminalFormat()
	}

	l15 := log15.New()
	// Use LvlDebug for the backend and filter levels within the adapter.
	// This allows for dynamic level changes without rebuilding the handler.
	handler := log15.LvlFilterHandler(log15.LvlDebug, log15.StreamHandler(aw, format))
	l15.SetHandler(handler)

	logger := &log15Logger{
		l:          l15,
		out:        aw,
		separator:  o.separator,
		withTrace:  o.withTrace,
		withCaller: o.withCaller,
		callerSkip: o.callerSkip,
	}
	logger.lvl.Store(int32(o.level))

	return logger, nil
}

func (l *log15Logger) log(level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	// Pre-allocate assuming 2 extra fields for source and stack.
	fields := make([]any, 0, len(keyValues)+4)
	fields = append(fields, l.processKeyValues(keyValues...)...)

	if l.withCaller {
		fields = append(fields, "source", caller.New(l.callerSkip+2).Location())
	}

	if l.withTrace && level >= unilog.ErrorLevel {
		fields = append(fields, "stack", string(debug.Stack()))
	}

	// Use the helper to process and append user-provided key-values.

	switch level {
	case unilog.DebugLevel:
		l.l.Debug(msg, fields...)
	case unilog.InfoLevel:
		l.l.Info(msg, fields...)
	case unilog.WarnLevel:
		l.l.Warn(msg, fields...)
	case unilog.ErrorLevel:
		l.l.Error(msg, fields...)
	case unilog.CriticalLevel:
		l.l.Crit(msg, fields...)
	case unilog.FatalLevel:
		l.l.Crit(msg, fields...)
		os.Exit(1)
	}
}

// Log implements the unilog.Logger interface for log15.
func (l *log15Logger) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(level, msg, 0, keyValues...)
}

// LogWithSkip implements the unilog.Logger interface for log15.
func (l *log15Logger) LogWithSkip(_ context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	l.log(level, msg, skip, keyValues...)
}

// Enabled checks if the given log level is enabled.
func (l *log15Logger) Enabled(level unilog.LogLevel) bool {
	return level >= unilog.LogLevel(l.lvl.Load())
}

// With returns a new logger with the provided keyValues added to the context.
func (l *log15Logger) With(keyValues ...any) unilog.Logger {
	if len(keyValues) < 2 {
		return l
	}

	clone := l.clone()
	clone.l = l.l.New(keyValues...)

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (l *log15Logger) WithGroup(name string) unilog.Logger {
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

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (l *log15Logger) SetLevel(level unilog.LogLevel) error {
	if err := unilog.ValidateLogLevel(level); err != nil {
		return err
	}
	l.lvl.Store(int32(level))
	return nil
}

// SetOutput changes the destination for log output.
func (l *log15Logger) SetOutput(w io.Writer) error {
	return l.out.Swap(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (l *log15Logger) CallerSkip() int {
	return l.callerSkip
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
func (l *log15Logger) WithCallerSkip(skip int) (unilog.Logger, error) {
	if skip < 0 {
		return l, unilog.ErrInvalidSourceSkip
	}

	clone := l.clone()
	clone.callerSkip = skip

	return clone, nil
}

// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
func (l *log15Logger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}
	return l.WithCallerSkip(l.callerSkip + delta)
}

// processKeyValues processes the given keyValues and returns a slice of alternating
// keys and values, applying the logger's key prefix.
func (l *log15Logger) processKeyValues(keyValues ...any) []any {
	if l.keyPrefix == "" {
		return keyValues
	}

	processedKVs := make([]any, len(keyValues))
	copy(processedKVs, keyValues)

	prefix := l.keyPrefix + l.separator
	for i := 0; i < len(processedKVs)-1; i += 2 {
		key, ok := processedKVs[i].(string)
		if !ok {
			key = fmt.Sprint(processedKVs[i])
		}
		processedKVs[i] = prefix + key
	}

	return processedKVs
}

// clone returns a deep copy of the logger.
func (l *log15Logger) clone() *log15Logger {
	clone := &log15Logger{
		l:          l.l, // log15.Logger is a struct with a pointer, shallow copy is fine
		out:        l.out,
		keyPrefix:  l.keyPrefix,
		separator:  l.separator,
		withTrace:  l.withTrace,
		withCaller: l.withCaller,
		callerSkip: l.callerSkip,
	}
	clone.lvl.Store(l.lvl.Load())
	return clone
}

// Clone returns a deep copy of the logger as a unilog.Logger.
func (l *log15Logger) Clone() unilog.Logger {
	return l.clone()
}

// Trace logs a message at the trace level.
func (l *log15Logger) Trace(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.TraceLevel, msg, 0, keyValues...)
}

// Debug logs a message at the debug level.
func (l *log15Logger) Debug(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.DebugLevel, msg, 0, keyValues...)
}

// Info logs a message at the info level.
func (l *log15Logger) Info(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.InfoLevel, msg, 0, keyValues...)
}

// Warn logs a message at the warn level.
func (l *log15Logger) Warn(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.WarnLevel, msg, 0, keyValues...)
}

// Error logs a message at the error level.
func (l *log15Logger) Error(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.ErrorLevel, msg, 0, keyValues...)
}

// Critical logs a message at the critical level.
func (l *log15Logger) Critical(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.CriticalLevel, msg, 0, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *log15Logger) Fatal(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.FatalLevel, msg, 0, keyValues...)
}

// Panic logs a message at the panic level and panics.
func (l *log15Logger) Panic(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.PanicLevel, msg, 0, keyValues...)
}
