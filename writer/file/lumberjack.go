package file

import (
	"gopkg.in/natefinch/lumberjack.v2"
)

// LumberjackConfig holds the configuration for a lumberjack-based writer.
// It exposes the most common lumberjack options.
type LumberjackConfig struct {
	// Filename is the file to write logs to.
	Filename string
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	MaxSize int
	// MaxAge is the maximum number of days to retain old log files based on timestamp.
	MaxAge int
	// MaxBackups is the maximum number of old log files to retain.
	MaxBackups int
	// Compress determines if the rotated log files should be compressed using gzip.
	Compress bool
}

// NewLumberjackWriter creates a new io.Writer that logs to a file and rotates
// it based on the provided configuration using the lumberjack library.
// The returned writer is safe for concurrent use.
func NewLumberjackWriter(config *LumberjackConfig) (*lumberjack.Logger, error) {
	// We can directly return a lumberjack.Logger because it already implements io.Writer.
	// This function acts as a convenient factory within your library.
	return &lumberjack.Logger{
		Filename:   config.Filename,
		MaxSize:    config.MaxSize,
		MaxAge:     config.MaxAge,
		MaxBackups: config.MaxBackups,
		Compress:   config.Compress,
	}, nil
}
