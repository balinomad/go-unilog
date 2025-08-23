package unilog

import (
	"fmt"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		name string
		l    LogLevel
		want string
	}{
		{"Debug level", LevelDebug, "DEBUG"},
		{"Info level", LevelInfo, "INFO"},
		{"Warn level", LevelWarn, "WARN"},
		{"Error level", LevelError, "ERROR"},
		{"Critical level", LevelCritical, "CRITICAL"},
		{"Fatal level", LevelFatal, "FATAL"},
		{"Below minimum level", LevelDebug - 1, "UNKNOWN (-1)"},
		{"Above maximum level", LevelFatal + 1, fmt.Sprintf("UNKNOWN (%d)", LevelFatal+1)},
		{"Far above maximum", 100, "UNKNOWN (100)"},
		{"Far below minimum", -100, "UNKNOWN (-100)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.String(); got != tt.want {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrInvalidLogLevel(t *testing.T) {
	tests := []struct {
		name       string
		level      LogLevel
		wantSubstr string
	}{
		{"Below minimum", LevelDebug - 1, "invalid log level -1"},
		{"Above maximum", LevelFatal + 1, fmt.Sprintf("invalid log level %d", LevelFatal+1)},
		{"Far above", 1234, "invalid log level 1234"},
		{"Far below", -999, "invalid log level -999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrInvalidLogLevel(tt.level)
			if err == nil {
				t.Errorf("ErrInvalidLogLevel() = nil, want error")
				return
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("ErrInvalidLogLevel() = %v, want substring %v", err.Error(), tt.wantSubstr)
			}
		})
	}
}

func TestIsValidLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level LogLevel
		want  bool
	}{
		{"Valid Debug", LevelDebug, true},
		{"Valid Info", LevelInfo, true},
		{"Valid Warn", LevelWarn, true},
		{"Valid Error", LevelError, true},
		{"Valid Critical", LevelCritical, true},
		{"Valid Fatal", LevelFatal, true},
		{"Below minimum", LevelDebug - 1, false},
		{"Above maximum", LevelFatal + 1, false},
		{"Far below minimum", -123, false},
		{"Far above maximum", 999, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidLogLevel(tt.level); got != tt.want {
				t.Errorf("IsValidLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   LogLevel
		wantErr bool
	}{
		{"Valid Debug", LevelDebug, false},
		{"Valid Fatal", LevelFatal, false},
		{"Valid middle (Warn)", LevelWarn, false},
		{"Below minimum", LevelDebug - 1, true},
		{"Above maximum", LevelFatal + 1, true},
		{"Far above", 500, true},
		{"Far below", -500, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLogLevel(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLogLevel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
