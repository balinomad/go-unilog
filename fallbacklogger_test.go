package unilog_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/balinomad/go-unilog"
)

func TestNewFallbackLogger(t *testing.T) {
	tests := []struct {
		name      string
		writer    io.Writer
		level     unilog.LogLevel
		wantErr   bool
		checkFunc func(t *testing.T, logger unilog.Logger)
	}{
		{
			name:    "creates logger with valid params",
			writer:  &bytes.Buffer{},
			level:   unilog.InfoLevel,
			wantErr: false,
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if logger == nil {
					t.Fatal("expected non-nil logger")
				}
				if !logger.Enabled(unilog.InfoLevel) {
					t.Error("logger should be enabled at InfoLevel")
				}
				if logger.Enabled(unilog.DebugLevel) {
					t.Error("logger should not be enabled below InfoLevel")
				}
			},
		},
		{
			name:    "creates logger with debug level",
			writer:  io.Discard,
			level:   unilog.DebugLevel,
			wantErr: false,
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if !logger.Enabled(unilog.DebugLevel) {
					t.Error("logger should be enabled at DebugLevel")
				}
			},
		},
		{
			name:    "creates logger with trace level",
			writer:  io.Discard,
			level:   unilog.TraceLevel,
			wantErr: false,
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if !logger.Enabled(unilog.TraceLevel) {
					t.Error("logger should be enabled at TraceLevel")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := unilog.NewFallbackLogger(tt.writer, tt.level)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewFallbackLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil && logger != nil {
				tt.checkFunc(t, logger)
			}
		})
	}
}

func TestFallbackLogger_Log(t *testing.T) {
	tests := []struct {
		name           string
		level          unilog.LogLevel
		logLevel       unilog.LogLevel
		msg            string
		keyValues      []any
		wantLogged     bool
		wantContains   []string
		wantNotContain string
	}{
		{
			name:       "does not log below minimum level",
			level:      unilog.InfoLevel,
			logLevel:   unilog.DebugLevel,
			msg:        "debug message",
			wantLogged: false,
		},
		{
			name:      "logs at minimum level",
			level:     unilog.InfoLevel,
			logLevel:  unilog.InfoLevel,
			msg:       "info message",
			keyValues: []any{"key1", "val1", "key2", 2},
			wantContains: []string{
				"INFO: info message",
				"key1=val1",
				"key2=2",
			},
			wantLogged: true,
		},
		{
			name:       "logs above minimum level",
			level:      unilog.InfoLevel,
			logLevel:   unilog.ErrorLevel,
			msg:        "error message",
			wantLogged: true,
			wantContains: []string{
				"ERROR: error message",
			},
		},
		{
			name:           "ignores odd number of keyValues",
			level:          unilog.WarnLevel,
			logLevel:       unilog.WarnLevel,
			msg:            "warn message",
			keyValues:      []any{"key1"},
			wantLogged:     true,
			wantNotContain: "key1=",
		},
		{
			name:       "logs without keyValues",
			level:      unilog.InfoLevel,
			logLevel:   unilog.InfoLevel,
			msg:        "simple message",
			wantLogged: true,
			wantContains: []string{
				"INFO: simple message",
			},
		},
		{
			name:       "logs with multiple keyValues",
			level:      unilog.DebugLevel,
			logLevel:   unilog.DebugLevel,
			msg:        "debug with attrs",
			keyValues:  []any{"a", 1, "b", 2, "c", 3},
			wantLogged: true,
			wantContains: []string{
				"DEBUG: debug with attrs",
				"a=1",
				"b=2",
				"c=3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger, err := unilog.NewFallbackLogger(buf, tt.level)
			if err != nil {
				t.Fatalf("NewFallbackLogger() error = %v", err)
			}

			ctx := context.Background()
			logger.Log(ctx, tt.logLevel, tt.msg, tt.keyValues...)

			output := buf.String()

			if tt.wantLogged && buf.Len() == 0 {
				t.Error("expected log output, got none")
			}

			if !tt.wantLogged && buf.Len() > 0 {
				t.Errorf("expected no log output, got: %s", output)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q, got: %s", want, output)
				}
			}

			if tt.wantNotContain != "" && strings.Contains(output, tt.wantNotContain) {
				t.Errorf("output should not contain %q, got: %s", tt.wantNotContain, output)
			}
		})
	}
}

func TestFallbackLogger_Enabled(t *testing.T) {
	tests := []struct {
		name        string
		configLevel unilog.LogLevel
		testLevel   unilog.LogLevel
		wantEnabled bool
	}{
		{
			name:        "debug disabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.DebugLevel,
			wantEnabled: false,
		},
		{
			name:        "info disabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.InfoLevel,
			wantEnabled: false,
		},
		{
			name:        "warn enabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.WarnLevel,
			wantEnabled: true,
		},
		{
			name:        "error enabled when warn configured",
			configLevel: unilog.WarnLevel,
			testLevel:   unilog.ErrorLevel,
			wantEnabled: true,
		},
		{
			name:        "trace enabled when trace configured",
			configLevel: unilog.TraceLevel,
			testLevel:   unilog.TraceLevel,
			wantEnabled: true,
		},
		{
			name:        "all enabled when trace configured",
			configLevel: unilog.TraceLevel,
			testLevel:   unilog.ErrorLevel,
			wantEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := unilog.NewFallbackLogger(io.Discard, tt.configLevel)
			if err != nil {
				t.Fatalf("NewFallbackLogger() error = %v", err)
			}

			got := logger.Enabled(tt.testLevel)
			if got != tt.wantEnabled {
				t.Errorf("Enabled(%v) = %v, want %v", tt.testLevel, got, tt.wantEnabled)
			}
		})
	}
}

func TestFallbackLogger_With(t *testing.T) {
	logger, err := unilog.NewFallbackLogger(io.Discard, unilog.InfoLevel)
	if err != nil {
		t.Fatalf("NewFallbackLogger() error = %v", err)
	}

	loggerWith := logger.With("key", "val")

	if logger != loggerWith {
		t.Error("With() should return the same instance for fallbackLogger")
	}
}

func TestFallbackLogger_WithGroup(t *testing.T) {
	logger, err := unilog.NewFallbackLogger(io.Discard, unilog.InfoLevel)
	if err != nil {
		t.Fatalf("NewFallbackLogger() error = %v", err)
	}

	loggerWithGroup := logger.WithGroup("group")

	if logger != loggerWithGroup {
		t.Error("WithGroup() should return the same instance for fallbackLogger")
	}
}

func TestFallbackLogger_SetLevel(t *testing.T) {
	tests := []struct {
		name         string
		initialLevel unilog.LogLevel
		newLevel     unilog.LogLevel
		wantErr      bool
		checkFunc    func(t *testing.T, logger unilog.Logger)
	}{
		{
			name:         "change from info to debug",
			initialLevel: unilog.InfoLevel,
			newLevel:     unilog.DebugLevel,
			wantErr:      false,
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if !logger.Enabled(unilog.DebugLevel) {
					t.Error("logger should be enabled at DebugLevel after SetLevel")
				}
			},
		},
		{
			name:         "change from debug to error",
			initialLevel: unilog.DebugLevel,
			newLevel:     unilog.ErrorLevel,
			wantErr:      false,
			checkFunc: func(t *testing.T, logger unilog.Logger) {
				if logger.Enabled(unilog.InfoLevel) {
					t.Error("logger should not be enabled below ErrorLevel")
				}
				if !logger.Enabled(unilog.ErrorLevel) {
					t.Error("logger should be enabled at ErrorLevel")
				}
			},
		},
		{
			name:         "invalid level above max",
			initialLevel: unilog.InfoLevel,
			newLevel:     unilog.MaxLevel + 1,
			wantErr:      true,
		},
		{
			name:         "invalid level below min",
			initialLevel: unilog.InfoLevel,
			newLevel:     unilog.MinLevel - 1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := unilog.NewFallbackLogger(io.Discard, tt.initialLevel)
			if err != nil {
				t.Fatalf("NewFallbackLogger() error = %v", err)
			}

			err = logger.SetLevel(tt.newLevel)

			if (err != nil) != tt.wantErr {
				t.Errorf("SetLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil && err == nil {
				tt.checkFunc(t, logger)
			}
		})
	}
}

func TestFallbackLogger_SetOutput(t *testing.T) {
	tests := []struct {
		name      string
		testMsg   string
		checkFunc func(t *testing.T, buf *bytes.Buffer)
	}{
		{
			name:    "output written to new writer",
			testMsg: "test message",
			checkFunc: func(t *testing.T, buf *bytes.Buffer) {
				if buf.Len() == 0 {
					t.Error("expected output in new writer, got none")
				}
				if !strings.Contains(buf.String(), "test message") {
					t.Errorf("expected message in output, got: %s", buf.String())
				}
			},
		},
		{
			name:    "multiple writes to new output",
			testMsg: "first",
			checkFunc: func(t *testing.T, buf *bytes.Buffer) {
				if !strings.Contains(buf.String(), "first") {
					t.Error("expected first message in output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := unilog.NewFallbackLogger(io.Discard, unilog.InfoLevel)
			if err != nil {
				t.Fatalf("NewFallbackLogger() error = %v", err)
			}

			newBuf := &bytes.Buffer{}
			err = logger.SetOutput(newBuf)
			if err != nil {
				t.Fatalf("SetOutput() error = %v", err)
			}

			logger.Info(context.Background(), tt.testMsg)

			if tt.checkFunc != nil {
				tt.checkFunc(t, newBuf)
			}
		})
	}
}

func TestFallbackLogger_LevelMethods(t *testing.T) {
	tests := []struct {
		name      string
		method    func(unilog.Logger, context.Context, string, ...any)
		wantLevel string
	}{
		{
			name:      "trace logs at trace level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Trace(ctx, msg, kv...) },
			wantLevel: "TRACE",
		},
		{
			name:      "debug logs at debug level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Debug(ctx, msg, kv...) },
			wantLevel: "DEBUG",
		},
		{
			name:      "info logs at info level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Info(ctx, msg, kv...) },
			wantLevel: "INFO",
		},
		{
			name:      "warn logs at warn level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Warn(ctx, msg, kv...) },
			wantLevel: "WARN",
		},
		{
			name:      "error logs at error level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Error(ctx, msg, kv...) },
			wantLevel: "ERROR",
		},
		{
			name:      "critical logs at critical level",
			method:    func(l unilog.Logger, ctx context.Context, msg string, kv ...any) { l.Critical(ctx, msg, kv...) },
			wantLevel: "CRITICAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger, err := unilog.NewFallbackLogger(buf, unilog.MinLevel)
			if err != nil {
				t.Fatalf("NewFallbackLogger() error = %v", err)
			}

			ctx := context.Background()
			tt.method(logger, ctx, "message")

			output := buf.String()
			expectedOutput := tt.wantLevel + ": message"
			if !strings.Contains(output, expectedOutput) {
				t.Errorf("expected output to contain %q, got: %s", expectedOutput, output)
			}
		})
	}
}

// Test for Fatal's call to os.Exit. This is a standard way to test this.
// It re-runs the test with a specific environment variable set.
func TestFallbackLogger_Fatal(t *testing.T) {
	if os.Getenv("BE_FATAL") == "1" {
		logger, _ := unilog.NewFallbackLogger(io.Discard, unilog.FatalLevel)
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
	logger, err := unilog.NewFallbackLogger(io.Discard, unilog.InfoLevel)
	if err != nil {
		t.Fatalf("NewFallbackLogger() error = %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic, got none")
		}
	}()

	logger.Panic(context.Background(), "panic message")
}

func TestFallbackLogger_NilContext(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := unilog.NewFallbackLogger(buf, unilog.InfoLevel)
	if err != nil {
		t.Fatalf("NewFallbackLogger() error = %v", err)
	}

	logger.Info(nil, "test with nil context")

	if buf.Len() == 0 {
		t.Error("expected log output with nil context")
	}
}
