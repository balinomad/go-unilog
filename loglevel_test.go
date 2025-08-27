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
		{"Trace level", TraceLevel, "TRACE"},
		{"Debug level", DebugLevel, "DEBUG"},
		{"Info level", InfoLevel, "INFO"},
		{"Warn level", WarnLevel, "WARN"},
		{"Error level", ErrorLevel, "ERROR"},
		{"Critical level", CriticalLevel, "CRITICAL"},
		{"Fatal level", FatalLevel, "FATAL"},
		{"Panic level", PanicLevel, "PANIC"},
		{"Below minimum level", MinLevel - 1, fmt.Sprintf("UNKNOWN (%d)", MinLevel-1)},
		{"Above maximum level", MaxLevel + 1, fmt.Sprintf("UNKNOWN (%d)", MaxLevel+1)},
		{"Far below minimum", MinLevel - 100, fmt.Sprintf("UNKNOWN (%d)", MinLevel-100)},
		{"Far above maximum", MaxLevel + 100, fmt.Sprintf("UNKNOWN (%d)", MaxLevel+100)},
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
		wantLevel LogLevel
		wantErr   bool
	}{
		{"Valid TRACE", "TRACE", TraceLevel, false},
		{"Valid DEBUG", "DEBUG", DebugLevel, false},
		{"Valid info (lowercase)", "info", InfoLevel, false},
		{"Valid WaRn (mixed case)", "WaRn", WarnLevel, false},
		{"Valid ERROR", "ERROR", ErrorLevel, false},
		{"Valid CRITICAL", "CRITICAL", CriticalLevel, false},
		{"Valid FATAL", "FATAL", FatalLevel, false},
		{"Valid PANIC", "PANIC", PanicLevel, false},
		{"Invalid level", "INVALID", InfoLevel, true},
		{"Empty string", "", InfoLevel, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, err := ParseLevel(tt.levelStr)
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
		level      LogLevel
		wantSubstr string
	}{
		{"Below minimum", MinLevel - 1, fmt.Sprintf("invalid log level %d", MinLevel-1)},
		{"Above maximum", MaxLevel + 1, fmt.Sprintf("invalid log level %d", MaxLevel+1)},
		{"Far below", MinLevel - 1000, fmt.Sprintf("invalid log level %d", MinLevel-1000)},
		{"Far above", MaxLevel + 1000, fmt.Sprintf("invalid log level %d", MaxLevel+1000)},
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
		{"Valid Trace", TraceLevel, true},
		{"Valid Debug", DebugLevel, true},
		{"Valid Info", InfoLevel, true},
		{"Valid Warn", WarnLevel, true},
		{"Valid Error", ErrorLevel, true},
		{"Valid Critical", CriticalLevel, true},
		{"Valid Fatal", FatalLevel, true},
		{"Valid Panic", PanicLevel, true},
		{"Below minimum", MinLevel - 1, false},
		{"Above maximum", MaxLevel + 1, false},
		{"Far below minimum", MinLevel - 999, false},
		{"Far above maximum", MaxLevel + 999, false},
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
		{"Valid Trace", TraceLevel, false},
		{"Valid Debug", DebugLevel, false},
		{"Valid Info", InfoLevel, false},
		{"Valid Warn", WarnLevel, false},
		{"Valid Error", ErrorLevel, false},
		{"Valid Critical", CriticalLevel, false},
		{"Valid Fatal", FatalLevel, false},
		{"Valid Panic", PanicLevel, false},
		{"Below minimum", MinLevel - 1, true},
		{"Above maximum", MaxLevel + 1, true},
		{"Far below minimum", MinLevel - 999, true},
		{"Far above maximum", MaxLevel + 999, true},
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
