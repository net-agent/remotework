package utils

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNamedLogger_BackwardsCompat(t *testing.T) {
	// NewNamedLogger 仍然可用，asyncOutput 参数被忽略
	logger := NewNamedLogger("test_sync", false)
	logger.Println("hello sync")

	loggerAsync := NewNamedLogger("test_async", true)
	loggerAsync.Printf("hello %s", "async")
	loggerAsync.Close()
}

func TestNamedLogger_EmptyNameCompat(t *testing.T) {
	logger := NewNamedLogger("", false)
	// 空名称 logger 不应 panic
	logger.Printf("noop %d", 1)
	logger.Println("noop")
	logger.Close()
}

func TestSetNamedLoggerOutputDist_Noop(t *testing.T) {
	// 调用不应 panic（已废弃，空操作）
	SetNamedLoggerOutputDist(nil)
}

func TestNewModuleLogger(t *testing.T) {
	log := NewModuleLogger("hub.svc")
	if log == nil {
		t.Fatal("NewModuleLogger returned nil")
	}
	// 不应 panic
	log.Info("test message", "key", "value")
}

func TestNamedLogger_Printf_Output(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	nl := &NamedLogger{Log: slog.New(fanout).With("module", "compat")}
	nl.Printf("hello %s %d", "world", 42)

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

func TestNamedLogger_Println_Output(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	nl := &NamedLogger{Log: slog.New(fanout).With("module", "mod")}
	nl.Println("hello world")

	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("Println message wrong, got:\n%s", out)
	}
}
