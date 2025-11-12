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
		{"trace level", handler.TraceLevel, "TRACE"},
		{"debug level", handler.DebugLevel, "DEBUG"},
		{"info level", handler.InfoLevel, "INFO"},
		{"warn level", handler.WarnLevel, "WARN"},
		{"error level", handler.ErrorLevel, "ERROR"},
		{"critical level", handler.CriticalLevel, "CRITICAL"},
		{"fatal level", handler.FatalLevel, "FATAL"},
		{"panic level", handler.PanicLevel, "PANIC"},
		{"below minimum level", handler.MinLevel - 1, fmt.Sprintf("UNKNOWN (%d)", handler.MinLevel-1)},
		{"above maximum level", handler.MaxLevel + 1, fmt.Sprintf("UNKNOWN (%d)", handler.MaxLevel+1)},
		{"far below minimum", handler.MinLevel - 100, fmt.Sprintf("UNKNOWN (%d)", handler.MinLevel-100)},
		{"far above maximum", handler.MaxLevel + 100, fmt.Sprintf("UNKNOWN (%d)", handler.MaxLevel+100)},
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
		{"valid TRACE", "TRACE", handler.TraceLevel, false},
		{"valid DEBUG", "DEBUG", handler.DebugLevel, false},
		{"valid info (lowercase)", "info", handler.InfoLevel, false},
		{"valid WaRn (mixed case)", "WaRn", handler.WarnLevel, false},
		{"valid ERROR", "ERROR", handler.ErrorLevel, false},
		{"valid CRITICAL", "CRITICAL", handler.CriticalLevel, false},
		{"valid FATAL", "FATAL", handler.FatalLevel, false},
		{"valid PANIC", "PANIC", handler.PanicLevel, false},
		{"invalid level", "INVALID", handler.InfoLevel, true},
		{"empty string", "", handler.InfoLevel, true},
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
		{"valid trace", handler.TraceLevel, true},
		{"valid debug", handler.DebugLevel, true},
		{"valid info", handler.InfoLevel, true},
		{"valid warn", handler.WarnLevel, true},
		{"valid error", handler.ErrorLevel, true},
		{"valid critical", handler.CriticalLevel, true},
		{"valid fatal", handler.FatalLevel, true},
		{"valid panic", handler.PanicLevel, true},
		{"below minimum", handler.MinLevel - 1, false},
		{"above maximum", handler.MaxLevel + 1, false},
		{"far below minimum", handler.MinLevel - 999, false},
		{"far above maximum", handler.MaxLevel + 999, false},
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
		{"valid trace", handler.TraceLevel, false},
		{"valid debug", handler.DebugLevel, false},
		{"valid info", handler.InfoLevel, false},
		{"valid warn", handler.WarnLevel, false},
		{"valid error", handler.ErrorLevel, false},
		{"valid critical", handler.CriticalLevel, false},
		{"valid fatal", handler.FatalLevel, false},
		{"valid panic", handler.PanicLevel, false},
		{"below minimum", handler.MinLevel - 1, true},
		{"above maximum", handler.MaxLevel + 1, true},
		{"far below minimum", handler.MinLevel - 999, true},
		{"far above maximum", handler.MaxLevel + 999, true},
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
		{"trace maps to TRACE_VAL", handler.TraceLevel, "TRACE_VAL"},
		{"debug maps to DEBUG_VAL", handler.DebugLevel, "DEBUG_VAL"},
		{"info maps to INFO_VAL", handler.InfoLevel, "INFO_VAL"},
		{"warn maps to WARN_VAL", handler.WarnLevel, "WARN_VAL"},
		{"error maps to ERROR_VAL", handler.ErrorLevel, "ERROR_VAL"},
		{"critical maps to CRITICAL_VAL", handler.CriticalLevel, "CRITICAL_VAL"},
		{"fatal maps to FATAL_VAL", handler.FatalLevel, "FATAL_VAL"},
		{"panic maps to PANIC_VAL", handler.PanicLevel, "PANIC_VAL"},
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
		{"way below MinLevel clamps to Trace", handler.MinLevel - 10, "TRACE_VAL"},
		{"just below MinLevel clamps to Trace", handler.MinLevel - 1, "TRACE_VAL"},
		// above MaxLevel -> should clamp to PanicLevel (MaxLevel)
		{"just above MaxLevel clamps to Panic", handler.MaxLevel + 1, "PANIC_VAL"},
		{"way above MaxLevel clamps to Panic", handler.MaxLevel + 100, "PANIC_VAL"},
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
