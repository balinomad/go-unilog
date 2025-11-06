package handler_test

import (
	"fmt"
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

// helper to build a string mapper used across tests
func newStringMapper() *handler.LevelMapper[string] {
	return handler.NewLevelMapper(
		"TRACE_VAL",
		"DEBUG_VAL",
		"INFO_VAL",
		"WARN_VAL",
		"ERROR_VAL",
		"CRITICAL_VAL",
		"FATAL_VAL",
		"PANIC_VAL",
	)
}

func TestLevelMapper_Map_AllDefinedLevels_String(t *testing.T) {
	m := newStringMapper()

	tests := []struct {
		name  string
		level handler.LogLevel
		want  string
	}{
		{"Trace maps to TRACE_VAL", handler.TraceLevel, "TRACE_VAL"},
		{"Debug maps to DEBUG_VAL", handler.DebugLevel, "DEBUG_VAL"},
		{"Info maps to INFO_VAL", handler.InfoLevel, "INFO_VAL"},
		{"Warn maps to WARN_VAL", handler.WarnLevel, "WARN_VAL"},
		{"Error maps to ERROR_VAL", handler.ErrorLevel, "ERROR_VAL"},
		{"Critical maps to CRITICAL_VAL", handler.CriticalLevel, "CRITICAL_VAL"},
		{"Fatal maps to FATAL_VAL", handler.FatalLevel, "FATAL_VAL"},
		{"Panic maps to PANIC_VAL", handler.PanicLevel, "PANIC_VAL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.Map(tc.level)
			if got != tc.want {
				t.Fatalf("Map(%v) = %q; want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestLevelMapper_Map_ClampsOutOfRange_String(t *testing.T) {
	m := newStringMapper()

	tests := []struct {
		name  string
		level handler.LogLevel
		want  string
	}{
		// below MinLevel -> should clamp to TraceLevel (MinLevel)
		{"Way below MinLevel clamps to Trace", handler.MinLevel - 10, "TRACE_VAL"},
		{"Just below MinLevel clamps to Trace", handler.MinLevel - 1, "TRACE_VAL"},
		// above MaxLevel -> should clamp to PanicLevel (MaxLevel)
		{"Just above MaxLevel clamps to Panic", handler.MaxLevel + 1, "PANIC_VAL"},
		{"Way above MaxLevel clamps to Panic", handler.MaxLevel + 100, "PANIC_VAL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.Map(tc.level)
			if got != tc.want {
				t.Fatalf("Map(%v) = %q; want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestLevelMapper_Map_IntType_CoversIndexingAndClamping(t *testing.T) {
	// use ints to ensure generics path and array indexing works for non-string types
	m := handler.NewLevelMapper(10, 11, 12, 13, 14, 15, 16, 17)

	// verify each defined level maps to the correct integer
	expected := map[handler.LogLevel]int{
		handler.TraceLevel:    10,
		handler.DebugLevel:    11,
		handler.InfoLevel:     12,
		handler.WarnLevel:     13,
		handler.ErrorLevel:    14,
		handler.CriticalLevel: 15,
		handler.FatalLevel:    16,
		handler.PanicLevel:    17,
	}

	for lvl, want := range expected {
		t.Run("level mapping "+lvl.String(), func(t *testing.T) {
			got := m.Map(lvl)
			if got != want {
				t.Fatalf("Map(%v) = %v; want %v", lvl, got, want)
			}
		})
	}

	// clamping tests for int mapper
	outOfRangeTests := []struct {
		name  string
		level handler.LogLevel
		want  int
	}{
		{"below min clamps to trace", handler.MinLevel - 5, 10},
		{"above max clamps to panic", handler.MaxLevel + 5, 17},
	}

	for _, tc := range outOfRangeTests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.Map(tc.level)
			if got != tc.want {
				t.Fatalf("Map(%v) = %v; want %v", tc.level, got, tc.want)
			}
		})
	}
}
