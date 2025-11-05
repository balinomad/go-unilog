package handler

import (
	"fmt"
	"strings"
)

// LogLevel represents log severity levels.
type LogLevel int32

// Log levels are ordered from least to most severe.
const (
	TraceLevel LogLevel = iota - 1
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	CriticalLevel
	FatalLevel
	PanicLevel

	MaxLevel     LogLevel = PanicLevel
	MinLevel     LogLevel = TraceLevel
	DefaultLevel LogLevel = InfoLevel
)

// String returns a human-readable representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case TraceLevel:
		return "TRACE"
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case CriticalLevel:
		return "CRITICAL"
	case FatalLevel:
		return "FATAL"
	case PanicLevel:
		return "PANIC"
	default:
		return fmt.Sprintf("UNKNOWN (%d)", l)
	}
}

// ParseLevel converts a string to a LogLevel.
// It is case-insensitive. If the string is not a valid level,
// it returns InfoLevel and an error.
func ParseLevel(levelStr string) (LogLevel, error) {
	switch strings.ToUpper(levelStr) {
	case "TRACE":
		return TraceLevel, nil
	case "DEBUG":
		return DebugLevel, nil
	case "INFO":
		return InfoLevel, nil
	case "WARN":
		return WarnLevel, nil
	case "ERROR":
		return ErrorLevel, nil
	case "CRITICAL":
		return CriticalLevel, nil
	case "FATAL":
		return FatalLevel, nil
	case "PANIC":
		return PanicLevel, nil
	}
	return DefaultLevel, fmt.Errorf("unknown log level: %q", levelStr)
}

// IsValidLogLevel returns true if the given log level is valid.
func IsValidLogLevel(level LogLevel) bool {
	return level >= MinLevel && level <= MaxLevel
}

// ValidateLogLevel returns an error if the given log level is invalid.
func ValidateLogLevel(level LogLevel) error {
	if !IsValidLogLevel(level) {
		return invalidLogLevelError(level)
	}

	return nil
}
