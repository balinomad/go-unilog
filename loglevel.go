package unilog

import "fmt"

// LogLevel represents log severity levels.
type LogLevel int32

// Log levels are ordered from least to most severe.
const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
	LevelFatal
)

// String returns a human-readable representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelCritical:
		return "CRITICAL"
	case LevelFatal:
		return "FATAL"
	default:
		return fmt.Sprintf("UNKNOWN (%d)", l)
	}
}

// ErrInvalidLogLevel is returned when a LogLevel is out of range.
func ErrInvalidLogLevel(level LogLevel) error {
	return fmt.Errorf("invalid log level %d, must be between %d and %d", level, LevelDebug, LevelFatal)
}

// IsValidLogLevel returns true if the given log level is valid.
func IsValidLogLevel(level LogLevel) bool {
	return level >= LevelDebug && level <= LevelFatal
}

// ValidateLogLevel returns an error if the given log level is invalid.
func ValidateLogLevel(level LogLevel) error {
	if !IsValidLogLevel(level) {
		return ErrInvalidLogLevel(level)
	}

	return nil
}
