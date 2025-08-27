package stdlog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"

	"github.com/balinomad/go-atomicwriter"
	"github.com/balinomad/go-caller"
	"github.com/balinomad/go-ctxmap"
	"github.com/balinomad/go-unilog"
)

// DefaultKeySeparator is the default separator for group key prefixes.
const DefaultKeySeparator = "_"

// fieldStringer returns a string representation of a key-value pair.
var fieldStringer = func(k string, v any) string { return k + "=" + fmt.Sprint(v) }

// stdLogger is a unilog.Logger implementation for Go's standard library log package.
type stdLogger struct {
	l          *log.Logger
	out        *atomicwriter.AtomicWriter
	lvl        atomic.Int32
	fields     *ctxmap.CtxMap
	withCaller bool
	withTrace  bool
	callerSkip int
}

// Ensure stdLogger implements the following interfaces.
var (
	_ unilog.Logger        = (*stdLogger)(nil)
	_ unilog.Configurator  = (*stdLogger)(nil)
	_ unilog.Cloner        = (*stdLogger)(nil)
	_ unilog.CallerSkipper = (*stdLogger)(nil)
)

// New creates a new unilog.Logger instance backed by the standard log.
func New(opts ...LogOption) (unilog.Logger, error) {
	o := &logOptions{
		level:     unilog.InfoLevel,
		output:    os.Stderr,
		separator: DefaultKeySeparator,
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

	l := &stdLogger{
		l:          log.New(aw, "", log.LstdFlags),
		out:        aw,
		fields:     ctxmap.NewCtxMap(o.separator, " ", fieldStringer),
		withCaller: o.withCaller,
		withTrace:  o.withTrace,
		callerSkip: o.callerSkip,
	}
	l.lvl.Store(int32(o.level))

	return l, nil
}

// log is the internal logging function used by the unilog.Logger interface. It adds caller and
// stack trace information before passing the record to the underlying slog logger.
func (l *stdLogger) log(level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	fields := l.fields.WithPairs(keyValues...)

	if l.withCaller {
		// Add 2 to skip this function and the caller function
		if s := l.callerSkip + skip + 2; s > 0 {
			fields.Set("source", caller.New(s).Location())
		}
	}

	if l.withTrace && level >= unilog.ErrorLevel {
		fields.Set("stack", string(debug.Stack()))
	}

	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(level.String())
	sb.WriteString("] ")
	sb.WriteString(msg)
	if fields.Len() > 0 {
		sb.WriteString(" ")
		sb.WriteString(fields.String())
	}

	l.l.Println(sb.String())

	// Handle termination levels
	switch level {
	case unilog.FatalLevel:
		os.Exit(1)
	case unilog.PanicLevel:
		panic(msg)
	}
}

// Log implements the unilog.Logger interface for the standard logger.
func (l *stdLogger) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(level, msg, 0, keyValues...)
}

// LogWithSkip implements the unilog.CallerSkipper interface for the standard logger.
func (l *stdLogger) LogWithSkip(_ context.Context, level unilog.LogLevel, msg string, skip int, keyValues ...any) {
	l.log(level, msg, skip, keyValues...)
}

// Enabled checks if the given log level is enabled.
func (l *stdLogger) Enabled(level unilog.LogLevel) bool {
	return level >= unilog.LogLevel(l.lvl.Load())
}

// With returns a new logger with the provided keyValues added to the context.
func (l *stdLogger) With(keyValues ...any) unilog.Logger {
	if len(keyValues) < 2 {
		return l
	}

	clone := l.clone()
	clone.fields = l.fields.WithPairs(keyValues)

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (l *stdLogger) WithGroup(name string) unilog.Logger {
	if name == "" {
		return l
	}

	clone := l.clone()
	clone.fields = l.fields.WithPrefix(name)

	return clone
}

// SetLevel dynamically changes the minimum level of logs that will be processed.
func (l *stdLogger) SetLevel(level unilog.LogLevel) error {
	if err := unilog.ValidateLogLevel(level); err != nil {
		return err
	}

	l.lvl.Store(int32(level))

	return nil
}

// SetOutput sets the log destination.
func (l *stdLogger) SetOutput(w io.Writer) error {
	return l.out.Swap(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (l *stdLogger) CallerSkip() int {
	return l.callerSkip
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
func (l *stdLogger) WithCallerSkip(skip int) (unilog.Logger, error) {
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
func (l *stdLogger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}

	return l.WithCallerSkip(l.callerSkip + delta)
}

// clone returns a deep copy of the logger.
func (l *stdLogger) clone() *stdLogger {
	clone := &stdLogger{
		l:          l.l,
		out:        l.out,
		withTrace:  l.withTrace,
		withCaller: l.withCaller,
		callerSkip: l.callerSkip,
	}
	clone.lvl.Store(l.lvl.Load())

	return clone
}

// Clone returns a deep copy of the logger as a unilog.Logger.
func (l *stdLogger) Clone() unilog.Logger {
	return l.clone()
}

// Trace logs a message at the trace level.
func (l *stdLogger) Trace(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.TraceLevel, msg, 0, keyValues...)
}

// Debug logs a message at the debug level.
func (l *stdLogger) Debug(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.DebugLevel, msg, 0, keyValues...)
}

// Info logs a message at the info level.
func (l *stdLogger) Info(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.InfoLevel, msg, 0, keyValues...)
}

// Warn logs a message at the warn level.
func (l *stdLogger) Warn(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.WarnLevel, msg, 0, keyValues...)
}

// Error logs a message at the error level.
func (l *stdLogger) Error(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.ErrorLevel, msg, 0, keyValues...)
}

// Critical logs a message at the critical level.
func (l *stdLogger) Critical(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.CriticalLevel, msg, 0, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *stdLogger) Fatal(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.FatalLevel, msg, 0, keyValues...)
}

// Panic logs a message at the panic level and panics.
func (l *stdLogger) Panic(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.PanicLevel, msg, 0, keyValues...)
}
