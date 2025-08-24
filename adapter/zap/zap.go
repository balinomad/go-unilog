package zap

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/balinomad/go-atomicwriter"
	"github.com/balinomad/go-unilog"
)

// internalSkipFrames is the fixed number of frames inside this adapter that must always be skipped.
const internalSkipFrames = 2

// zapLogger is a wrapper around Zap's logger.
type zapLogger struct {
	l          *zap.Logger
	lvl        zap.AtomicLevel
	out        *atomicwriter.AtomicWriter
	keyPrefix  string
	separator  string
	callerSkip int // Number of stack frames to skip, including internalSkipFrames
}

// Ensure zapLogger implements the following interfaces.
var (
	_ unilog.Logger        = (*zapLogger)(nil)
	_ unilog.Configurator  = (*zapLogger)(nil)
	_ unilog.Cloner        = (*zapLogger)(nil)
	_ unilog.CallerSkipper = (*zapLogger)(nil)
	_ unilog.Syncer        = (*zapLogger)(nil)
)

// New creates a new unilog.Logger instance backed by zap.
func New(opts ...ZapOption) (unilog.Logger, error) {
	o := &zapOptions{
		level:     unilog.LevelInfo,
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

	writeSyncer := zapcore.AddSync(aw)
	atomicLevel := zap.NewAtomicLevelAt(toZapLevel(o.level))

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	var encoder zapcore.Encoder
	if o.format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, writeSyncer, atomicLevel)

	// Build zap options natively.
	zapOpts := make([]zap.Option, 0, 2)
	if o.withCaller {
		// AddCallerSkip needs to account for our adapter's internal frames.
		zapOpts = append(zapOpts, zap.AddCaller(), zap.AddCallerSkip(o.callerSkip+internalSkipFrames))
	}
	if o.withTrace {
		// Adds stack trace to logs at Error level and above.
		zapOpts = append(zapOpts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	zl := zap.New(core, zapOpts...)

	logger := &zapLogger{
		l:          zl,
		lvl:        atomicLevel,
		out:        aw,
		separator:  o.separator,
		callerSkip: o.callerSkip,
	}

	return logger, nil
}

// Log implements the unilog.Logger interface for zap.
func (l *zapLogger) log(level unilog.LogLevel, msg string, keyValues ...any) {
	if !l.Enabled(level) {
		return
	}

	zLevel := toZapLevel(level)
	ce := l.l.Check(zLevel, msg)
	if ce == nil {
		return
	}

	ce.Write(l.processKeyValues(keyValues...)...)
}

// Log implements the unilog.Logger interface for zap.
func (l *zapLogger) Log(_ context.Context, level unilog.LogLevel, msg string, keyValues ...any) {
	l.log(level, msg, keyValues...)
}

// Enabled checks if the given log level is enabled.
func (l *zapLogger) Enabled(level unilog.LogLevel) bool {
	return l.lvl.Enabled(toZapLevel(level))
}

// With returns a new logger with the provided keyValues added to the context.
func (l *zapLogger) With(keyValues ...any) unilog.Logger {
	if len(keyValues) < 2 {
		return l
	}

	clone := l.clone()
	clone.l = l.l.With(l.processKeyValues(keyValues...)...)

	return clone
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
func (l *zapLogger) WithGroup(name string) unilog.Logger {
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
func (l *zapLogger) SetLevel(level unilog.LogLevel) error {
	if err := unilog.ValidateLogLevel(level); err != nil {
		return err
	}
	l.lvl.SetLevel(toZapLevel(level))

	return nil
}

// SetOutput changes the destination for log output.
func (l *zapLogger) SetOutput(w io.Writer) error {
	return l.out.Swap(w)
}

// CallerSkip returns the current number of stack frames being skipped.
func (l *zapLogger) CallerSkip() int {
	return l.callerSkip - internalSkipFrames
}

// WithCallerSkip returns a new Logger instance with the caller skip value updated.
func (l *zapLogger) WithCallerSkip(skip int) (unilog.Logger, error) {
	if skip < 0 {
		return l, unilog.ErrInvalidSourceSkip
	}

	return l.WithCallerSkipDelta(skip - l.CallerSkip())
}

// WithCallerSkipDelta returns a new Logger instance with the caller skip value altered by the given delta.
func (l *zapLogger) WithCallerSkipDelta(delta int) (unilog.Logger, error) {
	if delta == 0 {
		return l, nil
	}

	newCallerSkip := l.callerSkip + delta
	if newCallerSkip < internalSkipFrames {
		return l, unilog.ErrInvalidSourceSkip
	}

	clone := l.clone()
	clone.l = l.l.WithOptions(zap.AddCallerSkip(delta))
	clone.callerSkip = clone.callerSkip + delta

	return clone, nil
}

func (l *zapLogger) processKeyValues(keyValues ...any) []zap.Field {
	fields := make([]zap.Field, 0, len(keyValues)/2)
	prefix := ""
	if l.keyPrefix != "" {
		prefix = l.keyPrefix + l.separator
	}

	for i := 0; i < len(keyValues)-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprint(keyValues[i])
		}
		fields = append(fields, zap.Any(prefix+key, keyValues[i+1]))
	}
	return fields
}

// clone returns a deep copy of the logger.
func (l *zapLogger) clone() *zapLogger {
	return &zapLogger{
		l:          l.l,
		lvl:        l.lvl,
		out:        l.out,
		keyPrefix:  l.keyPrefix,
		separator:  l.separator,
		callerSkip: l.callerSkip,
	}
}

// Clone returns a deep copy of the logger as a unilog.Logger.
func (l *zapLogger) Clone() unilog.Logger {
	return l.clone()
}

// Sync flushes any buffered log entries.
func (l *zapLogger) Sync() error {
	return l.l.Sync()
}

// Debug logs a message at the debug level.
func (l *zapLogger) Debug(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelDebug, msg, keyValues...)
}

// Info logs a message at the info level.
func (l *zapLogger) Info(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelInfo, msg, keyValues...)
}

// Warn logs a message at the warn level.
func (l *zapLogger) Warn(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelWarn, msg, keyValues...)
}

// Error logs a message at the error level.
func (l *zapLogger) Error(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelError, msg, keyValues...)
}

// Critical logs a message at the critical level.
func (l *zapLogger) Critical(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelCritical, msg, keyValues...)
}

// Fatal logs a message at the fatal level and exits the process.
func (l *zapLogger) Fatal(_ context.Context, msg string, keyValues ...any) {
	l.log(unilog.LevelFatal, msg, keyValues...)
}

func toZapLevel(level unilog.LogLevel) zapcore.Level {
	switch level {
	case unilog.LevelDebug:
		return zapcore.DebugLevel
	case unilog.LevelInfo:
		return zapcore.InfoLevel
	case unilog.LevelWarn:
		return zapcore.WarnLevel
	case unilog.LevelError:
		return zapcore.ErrorLevel
	case unilog.LevelCritical:
		return zapcore.DPanicLevel
	case unilog.LevelFatal:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}
