package handler_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/balinomad/go-unilog/handler"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		name string
		l    handler.LogLevel
		want string
	}{
		{"Trace level", handler.TraceLevel, "TRACE"},
		{"Debug level", handler.DebugLevel, "DEBUG"},
		{"Info level", handler.InfoLevel, "INFO"},
		{"Warn level", handler.WarnLevel, "WARN"},
		{"Error level", handler.ErrorLevel, "ERROR"},
		{"Critical level", handler.CriticalLevel, "CRITICAL"},
		{"Fatal level", handler.FatalLevel, "FATAL"},
		{"Panic level", handler.PanicLevel, "PANIC"},
		{"Below minimum level", handler.MinLevel - 1, fmt.Sprintf("UNKNOWN (%d)", handler.MinLevel-1)},
		{"Above maximum level", handler.MaxLevel + 1, fmt.Sprintf("UNKNOWN (%d)", handler.MaxLevel+1)},
		{"Far below minimum", handler.MinLevel - 100, fmt.Sprintf("UNKNOWN (%d)", handler.MinLevel-100)},
		{"Far above maximum", handler.MaxLevel + 100, fmt.Sprintf("UNKNOWN (%d)", handler.MaxLevel+100)},
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
		wantLevel handler.LogLevel
		wantErr   bool
	}{
		{"Valid TRACE", "TRACE", handler.TraceLevel, false},
		{"Valid DEBUG", "DEBUG", handler.DebugLevel, false},
		{"Valid info (lowercase)", "info", handler.InfoLevel, false},
		{"Valid WaRn (mixed case)", "WaRn", handler.WarnLevel, false},
		{"Valid ERROR", "ERROR", handler.ErrorLevel, false},
		{"Valid CRITICAL", "CRITICAL", handler.CriticalLevel, false},
		{"Valid FATAL", "FATAL", handler.FatalLevel, false},
		{"Valid PANIC", "PANIC", handler.PanicLevel, false},
		{"Invalid level", "INVALID", handler.InfoLevel, true},
		{"Empty string", "", handler.InfoLevel, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, err := handler.ParseLevel(tt.levelStr)
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
		level      handler.LogLevel
		wantSubstr string
	}{
		{"Below minimum", handler.MinLevel - 1, fmt.Sprintf("invalid log level %d", handler.MinLevel-1)},
		{"Above maximum", handler.MaxLevel + 1, fmt.Sprintf("invalid log level %d", handler.MaxLevel+1)},
		{"Far below", handler.MinLevel - 1000, fmt.Sprintf("invalid log level %d", handler.MinLevel-1000)},
		{"Far above", handler.MaxLevel + 1000, fmt.Sprintf("invalid log level %d", handler.MaxLevel+1000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.XInvalidLogLevelError(tt.level)
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
		level handler.LogLevel
		want  bool
	}{
		{"Valid Trace", handler.TraceLevel, true},
		{"Valid Debug", handler.DebugLevel, true},
		{"Valid Info", handler.InfoLevel, true},
		{"Valid Warn", handler.WarnLevel, true},
		{"Valid Error", handler.ErrorLevel, true},
		{"Valid Critical", handler.CriticalLevel, true},
		{"Valid Fatal", handler.FatalLevel, true},
		{"Valid Panic", handler.PanicLevel, true},
		{"Below minimum", handler.MinLevel - 1, false},
		{"Above maximum", handler.MaxLevel + 1, false},
		{"Far below minimum", handler.MinLevel - 999, false},
		{"Far above maximum", handler.MaxLevel + 999, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handler.IsValidLogLevel(tt.level); got != tt.want {
				t.Errorf("IsValidLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   handler.LogLevel
		wantErr bool
	}{
		{"Valid Trace", handler.TraceLevel, false},
		{"Valid Debug", handler.DebugLevel, false},
		{"Valid Info", handler.InfoLevel, false},
		{"Valid Warn", handler.WarnLevel, false},
		{"Valid Error", handler.ErrorLevel, false},
		{"Valid Critical", handler.CriticalLevel, false},
		{"Valid Fatal", handler.FatalLevel, false},
		{"Valid Panic", handler.PanicLevel, false},
		{"Below minimum", handler.MinLevel - 1, true},
		{"Above maximum", handler.MaxLevel + 1, true},
		{"Far below minimum", handler.MinLevel - 999, true},
		{"Far above maximum", handler.MaxLevel + 999, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateLogLevel(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLogLevel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
