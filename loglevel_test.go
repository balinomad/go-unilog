package unilog_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/balinomad/go-unilog"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		name string
		l    unilog.LogLevel
		want string
	}{
		{"Trace level", unilog.TraceLevel, "TRACE"},
		{"Debug level", unilog.DebugLevel, "DEBUG"},
		{"Info level", unilog.InfoLevel, "INFO"},
		{"Warn level", unilog.WarnLevel, "WARN"},
		{"Error level", unilog.ErrorLevel, "ERROR"},
		{"Critical level", unilog.CriticalLevel, "CRITICAL"},
		{"Fatal level", unilog.FatalLevel, "FATAL"},
		{"Panic level", unilog.PanicLevel, "PANIC"},
		{"Below minimum level", unilog.MinLevel - 1, fmt.Sprintf("UNKNOWN (%d)", unilog.MinLevel-1)},
		{"Above maximum level", unilog.MaxLevel + 1, fmt.Sprintf("UNKNOWN (%d)", unilog.MaxLevel+1)},
		{"Far below minimum", unilog.MinLevel - 100, fmt.Sprintf("UNKNOWN (%d)", unilog.MinLevel-100)},
		{"Far above maximum", unilog.MaxLevel + 100, fmt.Sprintf("UNKNOWN (%d)", unilog.MaxLevel+100)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.String(); got != tt.want {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name      string
		levelStr  string
		wantLevel unilog.LogLevel
		wantErr   bool
	}{
		{"Valid TRACE", "TRACE", unilog.TraceLevel, false},
		{"Valid DEBUG", "DEBUG", unilog.DebugLevel, false},
		{"Valid info (lowercase)", "info", unilog.InfoLevel, false},
		{"Valid WaRn (mixed case)", "WaRn", unilog.WarnLevel, false},
		{"Valid ERROR", "ERROR", unilog.ErrorLevel, false},
		{"Valid CRITICAL", "CRITICAL", unilog.CriticalLevel, false},
		{"Valid FATAL", "FATAL", unilog.FatalLevel, false},
		{"Valid PANIC", "PANIC", unilog.PanicLevel, false},
		{"Invalid level", "INVALID", unilog.InfoLevel, true},
		{"Empty string", "", unilog.InfoLevel, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, err := unilog.ParseLevel(tt.levelStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotLevel != tt.wantLevel {
				t.Errorf("ParseLevel() gotLevel = %v, want %v", gotLevel, tt.wantLevel)
			}
		})
	}
}

func TestErrInvalidLogLevel(t *testing.T) {
	tests := []struct {
		name       string
		level      unilog.LogLevel
		wantSubstr string
	}{
		{"Below minimum", unilog.MinLevel - 1, fmt.Sprintf("invalid log level %d", unilog.MinLevel-1)},
		{"Above maximum", unilog.MaxLevel + 1, fmt.Sprintf("invalid log level %d", unilog.MaxLevel+1)},
		{"Far below", unilog.MinLevel - 1000, fmt.Sprintf("invalid log level %d", unilog.MinLevel-1000)},
		{"Far above", unilog.MaxLevel + 1000, fmt.Sprintf("invalid log level %d", unilog.MaxLevel+1000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := unilog.ErrInvalidLogLevel(tt.level)
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
		level unilog.LogLevel
		want  bool
	}{
		{"Valid Trace", unilog.TraceLevel, true},
		{"Valid Debug", unilog.DebugLevel, true},
		{"Valid Info", unilog.InfoLevel, true},
		{"Valid Warn", unilog.WarnLevel, true},
		{"Valid Error", unilog.ErrorLevel, true},
		{"Valid Critical", unilog.CriticalLevel, true},
		{"Valid Fatal", unilog.FatalLevel, true},
		{"Valid Panic", unilog.PanicLevel, true},
		{"Below minimum", unilog.MinLevel - 1, false},
		{"Above maximum", unilog.MaxLevel + 1, false},
		{"Far below minimum", unilog.MinLevel - 999, false},
		{"Far above maximum", unilog.MaxLevel + 999, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unilog.IsValidLogLevel(tt.level); got != tt.want {
				t.Errorf("IsValidLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   unilog.LogLevel
		wantErr bool
	}{
		{"Valid Trace", unilog.TraceLevel, false},
		{"Valid Debug", unilog.DebugLevel, false},
		{"Valid Info", unilog.InfoLevel, false},
		{"Valid Warn", unilog.WarnLevel, false},
		{"Valid Error", unilog.ErrorLevel, false},
		{"Valid Critical", unilog.CriticalLevel, false},
		{"Valid Fatal", unilog.FatalLevel, false},
		{"Valid Panic", unilog.PanicLevel, false},
		{"Below minimum", unilog.MinLevel - 1, true},
		{"Above maximum", unilog.MaxLevel + 1, true},
		{"Far below minimum", unilog.MinLevel - 999, true},
		{"Far above maximum", unilog.MaxLevel + 999, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := unilog.ValidateLogLevel(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLogLevel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
