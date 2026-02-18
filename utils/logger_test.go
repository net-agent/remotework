package utils

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	level := &slog.LevelVar{}
	level.Set(slog.LevelDebug)
	h := NewConsoleHandler(&buf, level)
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	logger := &Logger{
		sl:   slog.New(fanout).With("module", "test"),
		name: "test",
	}

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	out := buf.String()
	for _, want := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing level %q, got:\n%s", want, out)
		}
	}
	for _, want := range []string{"debug msg", "info msg", "warn msg", "error msg"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing message %q, got:\n%s", want, out)
		}
	}
}

func TestLogger_Printf(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	logger := &Logger{
		sl:   slog.New(fanout).With("module", "compat"),
		name: "compat",
	}

	logger.Printf("hello %s %d", "world", 42)

	out := buf.String()
	if !strings.Contains(out, "INFO") {
		t.Errorf("Printf should map to INFO, got:\n%s", out)
	}
	if !strings.Contains(out, "hello world 42") {
		t.Errorf("Printf message wrong, got:\n%s", out)
	}
	if !strings.Contains(out, "[compat]") {
		t.Errorf("Printf should include module name, got:\n%s", out)
	}
}

func TestLogger_Println(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	logger := &Logger{
		sl:   slog.New(fanout).With("module", "mod"),
		name: "mod",
	}

	logger.Println("hello world")

	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("Println message wrong, got:\n%s", out)
	}
}

func TestLogger_NilSafe(t *testing.T) {
	// 空模块名 logger 不应 panic
	logger := NewLogger("")
	logger.Debug("noop")
	logger.Info("noop")
	logger.Warn("noop")
	logger.Error("noop")
	logger.Printf("noop %d", 1)
	logger.Println("noop")
}

func TestLogger_StructuredArgs(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	logger := &Logger{
		sl:   slog.New(fanout).With("module", "svc"),
		name: "svc",
	}

	logger.Info("service stopped", "name", "rdp", "err", nil)

	out := buf.String()
	if !strings.Contains(out, "name=rdp") {
		t.Errorf("structured arg missing, got:\n%s", out)
	}
	if !strings.Contains(out, "service stopped") {
		t.Errorf("message missing, got:\n%s", out)
	}
}

func TestNewLogger_Module(t *testing.T) {
	logger := NewLogger("hub.svc")
	if logger.Name() != "hub.svc" {
		t.Errorf("Name() = %q, want %q", logger.Name(), "hub.svc")
	}
}
