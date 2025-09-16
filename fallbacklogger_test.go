package unilog

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNewFallbackLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := newFallbackLogger(buf, InfoLevel)
	if err != nil {
		t.Fatalf("newFallbackLogger() error = %v", err)
	}
	if logger == nil {
		t.Fatal("newFallbackLogger() returned nil logger")
	}
	if logger.l.Writer() != logger.w {
		t.Error("log.Logger writer not set to atomic writer")
	}
	if LogLevel(logger.lvl.Load()) != InfoLevel {
		t.Errorf("expected level %v, got %v", InfoLevel, LogLevel(logger.lvl.Load()))
	}
}

func TestFallbackLogger_Log(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, _ := newFallbackLogger(buf, InfoLevel)
	ctx := context.Background()

	// This should not be logged
	logger.Log(ctx, DebugLevel, "debug message")
	if buf.Len() > 0 {
		t.Errorf("logged message below minimum level: %s", buf.String())
	}

	// This should be logged
	logger.Log(ctx, InfoLevel, "info message", "key1", "val1", "key2", 2)
	output := buf.String()
	if !strings.Contains(output, "INFO: info message") {
		t.Errorf("log output missing level and message: %s", output)
	}
	if !strings.Contains(output, "key1=val1") {
		t.Errorf("log output missing key-value pair: %s", output)
	}
	if !strings.Contains(output, "key2=2") {
		t.Errorf("log output missing key-value pair: %s", output)
	}

	// Test odd number of keyValues
	buf.Reset()
	logger.Log(ctx, WarnLevel, "warn message", "key1")
	output = buf.String()
	if strings.Contains(output, "key1=") {
		t.Errorf("should not log incomplete key-value pair: %s", output)
	}
}

func TestFallbackLogger_Enabled(t *testing.T) {
	logger := &fallbackLogger{}
	logger.lvl.Store(int32(WarnLevel))

	if logger.Enabled(DebugLevel) {
		t.Error("Enabled(DebugLevel) should be false")
	}
	if logger.Enabled(InfoLevel) {
		t.Error("Enabled(InfoLevel) should be false")
	}
	if !logger.Enabled(WarnLevel) {
		t.Error("Enabled(WarnLevel) should be true")
	}
	if !logger.Enabled(ErrorLevel) {
		t.Error("Enabled(ErrorLevel) should be true")
	}
}

func TestFallbackLogger_With(t *testing.T) {
	logger, _ := newFallbackLogger(io.Discard, InfoLevel)
	loggerWith := logger.With("key", "val")

	if logger != loggerWith {
		t.Error("With() should return the same instance for fallbackLogger")
	}
}

func TestFallbackLogger_WithGroup(t *testing.T) {
	logger, _ := newFallbackLogger(io.Discard, InfoLevel)
	loggerWithGroup := logger.WithGroup("group")

	if logger != loggerWithGroup {
		t.Error("WithGroup() should return the same instance for fallbackLogger")
	}
}

func TestFallbackLogger_SetLevel(t *testing.T) {
	logger, _ := newFallbackLogger(io.Discard, InfoLevel)

	err := logger.SetLevel(DebugLevel)
	if err != nil {
		t.Fatalf("SetLevel() error = %v", err)
	}
	if LogLevel(logger.lvl.Load()) != DebugLevel {
		t.Errorf("level not updated, want %v, got %v", DebugLevel, LogLevel(logger.lvl.Load()))
	}

	err = logger.SetLevel(MaxLevel + 1)
	if err == nil {
		t.Error("SetLevel() expected an error for invalid level, got nil")
	}
}

func TestFallbackLogger_SetOutput(t *testing.T) {
	logger, _ := newFallbackLogger(io.Discard, InfoLevel)

	newBuf := &bytes.Buffer{}
	err := logger.SetOutput(newBuf)
	if err != nil {
		t.Fatalf("SetOutput() error = %v", err)
	}

	logger.Info(context.Background(), "test")
	if newBuf.Len() == 0 {
		t.Error("log output was not written to the new writer")
	}
}

func TestFallbackLogger_LevelMethods(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
		}
	}()

	buf := &bytes.Buffer{}
	logger, _ := newFallbackLogger(buf, MinLevel)
	ctx := context.Background()

	tests := []struct {
		name   string
		method func(ctx context.Context, msg string, keyValues ...any)
		level  string
	}{
		{"Trace", logger.Trace, "TRACE"},
		{"Debug", logger.Debug, "DEBUG"},
		{"Info", logger.Info, "INFO"},
		{"Warn", logger.Warn, "WARN"},
		{"Error", logger.Error, "ERROR"},
		{"Critical", logger.Critical, "CRITICAL"},
		// We test Fatal and Panic separately because they exit the process
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.method(ctx, "message")
			output := buf.String()
			if !strings.Contains(output, tt.level+": message") {
				t.Errorf("%s() did not log expected output, got: %s", tt.name, output)
			}
		})
	}
}

// Test for Fatal's call to os.Exit. This is a standard way to test this.
// It re-runs the test with a specific environment variable set.
func TestFallbackLogger_Fatal(t *testing.T) {
	if os.Getenv("BE_FATAL") == "1" {
		logger, _ := newFallbackLogger(io.Discard, FatalLevel)
		logger.Fatal(context.Background(), "fatal message")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestFallbackLogger_Fatal$")
	cmd.Env = append(os.Environ(), "BE_FATAL=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		// The program exited with a non-zero status, which is what we expect from os.Exit(1)
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

// Test for Panic's call to panic.
func TestFallbackLogger_Panic(t *testing.T) {
	logger, _ := newFallbackLogger(io.Discard, InfoLevel)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic, got none")
		}
	}()
	logger.Panic(context.Background(), "panic message")
}
