package utils

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestConsoleHandler_Format(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	logger := slog.New(h).With("module", "hub.svc")

	logger.Info("service stopped", "name", "rdp", "err", "none")

	out := buf.String()
	// 检查格式各部分
	if !strings.Contains(out, "INFO") {
		t.Errorf("missing level, got:\n%s", out)
	}
	if !strings.Contains(out, "[hub.svc]") {
		t.Errorf("missing module, got:\n%s", out)
	}
	if !strings.Contains(out, "service stopped") {
		t.Errorf("missing message, got:\n%s", out)
	}
	if !strings.Contains(out, "name=rdp") {
		t.Errorf("missing attr name=rdp, got:\n%s", out)
	}
	if !strings.Contains(out, "err=none") {
		t.Errorf("missing attr err=none, got:\n%s", out)
	}
}

func TestConsoleHandler_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	level := &slog.LevelVar{}
	level.Set(slog.LevelWarn)
	h := NewConsoleHandler(&buf, level)
	logger := slog.New(h).With("module", "test")

	logger.Info("should not appear")
	logger.Warn("should appear")

	out := buf.String()
	if strings.Contains(out, "should not appear") {
		t.Errorf("Info should be filtered at Warn level, got:\n%s", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Errorf("Warn should pass at Warn level, got:\n%s", out)
	}
}

func TestConsoleHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	h2 := h.WithAttrs([]slog.Attr{slog.String("module", "pre")})
	logger := slog.New(h2)

	logger.Info("test msg", "extra", "val")

	out := buf.String()
	if !strings.Contains(out, "[pre]") {
		t.Errorf("WithAttrs module missing, got:\n%s", out)
	}
	if !strings.Contains(out, "extra=val") {
		t.Errorf("extra attr missing, got:\n%s", out)
	}
}

func TestConsoleHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	h2 := h.WithGroup("grp").WithAttrs([]slog.Attr{slog.String("module", "mod")})
	logger := slog.New(h2)

	logger.Info("grouped")

	out := buf.String()
	if !strings.Contains(out, "[grp.mod]") {
		t.Errorf("WithGroup module prefix missing, got:\n%s", out)
	}
}

func TestFanoutHandler_MultiTarget(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := NewConsoleHandler(&buf1, &slog.LevelVar{})
	h2 := NewConsoleHandler(&buf2, &slog.LevelVar{})

	fanout := NewFanoutHandler()
	fanout.Add("a", h1)
	fanout.Add("b", h2)

	logger := slog.New(fanout).With("module", "multi")
	logger.Info("broadcast")

	if !strings.Contains(buf1.String(), "broadcast") {
		t.Error("handler a did not receive message")
	}
	if !strings.Contains(buf2.String(), "broadcast") {
		t.Error("handler b did not receive message")
	}
}

func TestFanoutHandler_AddRemove(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := NewConsoleHandler(&buf1, &slog.LevelVar{})
	h2 := NewConsoleHandler(&buf2, &slog.LevelVar{})

	fanout := NewFanoutHandler()
	fanout.Add("a", h1)
	fanout.Add("b", h2)

	slog.New(fanout).Info("msg1")

	if !strings.Contains(buf1.String(), "msg1") {
		t.Error("handler a should receive msg1")
	}
	if !strings.Contains(buf2.String(), "msg1") {
		t.Error("handler b should receive msg1")
	}

	// 移除 b，直接通过 fanout 发送
	fanout.Remove("b")
	buf1.Reset()
	buf2.Reset()
	slog.New(fanout).Info("msg2")

	if !strings.Contains(buf1.String(), "msg2") {
		t.Error("handler a should still receive after Remove(b)")
	}
	if strings.Contains(buf2.String(), "msg2") {
		t.Error("handler b should not receive after Remove")
	}
}

func TestFanoutHandler_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	fanout := NewFanoutHandler()
	fanout.Add("c", h)

	logger := slog.New(fanout).With("module", "conc")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.Info("concurrent", "i", i)
		}(i)
	}
	wg.Wait()

	lines := strings.Count(buf.String(), "\n")
	if lines != 100 {
		t.Errorf("expected 100 lines, got %d", lines)
	}
}

func TestFanoutHandler_Enabled(t *testing.T) {
	level := &slog.LevelVar{}
	level.Set(slog.LevelError)
	h := NewConsoleHandler(&bytes.Buffer{}, level)

	fanout := NewFanoutHandler()
	fanout.Add("strict", h)

	if fanout.Enabled(nil, slog.LevelInfo) {
		t.Error("should not be enabled for Info when only Error handler exists")
	}
	if !fanout.Enabled(nil, slog.LevelError) {
		t.Error("should be enabled for Error")
	}
}

func TestFanoutHandler_Empty(t *testing.T) {
	fanout := NewFanoutHandler()
	if fanout.Enabled(nil, slog.LevelError) {
		t.Error("empty fanout should not be enabled")
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &globalLevel)
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	logger := slog.New(fanout).With("module", "lvl")

	// 默认 Info 级别
	SetLevel(slog.LevelInfo)
	logger.Debug("hidden")
	if strings.Contains(buf.String(), "hidden") {
		t.Error("Debug should be hidden at Info level")
	}

	logger.Info("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Error("Info should be visible at Info level")
	}

	// 切换到 Debug
	buf.Reset()
	SetLevel(slog.LevelDebug)
	logger.Debug("now visible")
	if !strings.Contains(buf.String(), "now visible") {
		t.Error("Debug should be visible at Debug level")
	}

	// 恢复默认
	SetLevel(slog.LevelInfo)
}

func TestNamedLogger_Compat(t *testing.T) {
	var buf bytes.Buffer
	h := NewConsoleHandler(&buf, &slog.LevelVar{})
	fanout := NewFanoutHandler()
	fanout.Add("test", h)

	nl := &NamedLogger{Log: slog.New(fanout).With("module", "compat")}

	nl.Printf("hello %s", "world")
	nl.Println("goodbye")
	nl.Close() // 应该是空操作

	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("Printf compat failed, got:\n%s", out)
	}
	if !strings.Contains(out, "goodbye") {
		t.Errorf("Println compat failed, got:\n%s", out)
	}
}

func TestNamedLogger_EmptyName(t *testing.T) {
	nl := NewNamedLogger("", false)
	// 不应 panic
	nl.Printf("noop")
	nl.Println("noop")
	nl.Close()
}

func TestSetFormat(t *testing.T) {
	// 切换到 JSON 不应 panic
	SetFormat("json")
	// 切换回 text
	SetFormat("text")
}
