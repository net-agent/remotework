package utils

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// ConsoleHandler 人类可读的控制台日志 Handler
type ConsoleHandler struct {
	out   io.Writer
	level slog.Leveler
	mu    *sync.Mutex
	attrs []slog.Attr
	group string
}

// NewConsoleHandler 创建控制台 Handler
func NewConsoleHandler(out io.Writer, level slog.Leveler) *ConsoleHandler {
	return &ConsoleHandler{
		out:   out,
		level: level,
		mu:    &sync.Mutex{},
	}
}

func (h *ConsoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *ConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	// 格式: 2026/02/18 21:40:25 INFO  [module] message key=value ...
	var buf []byte

	// 时间戳
	buf = append(buf, r.Time.Format("2006/01/02 15:04:05")...)
	buf = append(buf, ' ')

	// 级别（5字符对齐）
	buf = append(buf, formatLevel(r.Level)...)
	buf = append(buf, ' ')

	// 模块名（从 attrs 中提取 "module" 字段）
	module := ""
	var otherAttrs []slog.Attr

	// 先收集预设 attrs
	for _, a := range h.attrs {
		if a.Key == "module" {
			module = a.Value.String()
		} else {
			otherAttrs = append(otherAttrs, a)
		}
	}

	// 再收集 record 中的 attrs
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "module" {
			module = a.Value.String()
		} else {
			otherAttrs = append(otherAttrs, a)
		}
		return true
	})

	if module != "" {
		buf = append(buf, '[')
		if h.group != "" {
			buf = appendLogText(buf, h.group)
			buf = append(buf, '.')
		}
		buf = appendLogText(buf, module)
		buf = append(buf, ']')
		buf = append(buf, ' ')
	}

	// 消息（转义换行等控制字符，防止用户数据伪造日志行）
	buf = appendLogText(buf, r.Message)

	// 结构化 key=value
	for _, a := range otherAttrs {
		buf = append(buf, ' ')
		buf = appendAttr(buf, h.group, a)
	}

	buf = append(buf, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf)
	return err
}

func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ConsoleHandler{
		out:   h.out,
		level: h.level,
		mu:    h.mu,
		attrs: append(cloneAttrs(h.attrs), attrs...),
		group: h.group,
	}
}

func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &ConsoleHandler{
		out:   h.out,
		level: h.level,
		mu:    h.mu,
		attrs: cloneAttrs(h.attrs),
		group: g,
	}
}

func formatLevel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN "
	case l >= slog.LevelInfo:
		return "INFO "
	default:
		return "DEBUG"
	}
}

func appendAttr(buf []byte, group string, a slog.Attr) []byte {
	if group != "" {
		buf = appendLogText(buf, group)
		buf = append(buf, '.')
	}
	// key 通常由代码提供，仍转义控制字符以封死注入路径
	buf = appendLogText(buf, a.Key)
	buf = append(buf, '=')
	buf = appendValue(buf, a.Value)
	return buf
}

func appendValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		buf = appendStringValue(buf, v.String())
	case slog.KindTime:
		buf = append(buf, v.Time().Format(time.RFC3339)...)
	case slog.KindDuration:
		buf = append(buf, v.Duration().String()...)
	case slog.KindGroup:
		attrs := v.Group()
		for i, a := range attrs {
			if i > 0 {
				buf = append(buf, ' ')
			}
			buf = appendAttr(buf, "", a)
		}
	default:
		buf = appendStringValue(buf, fmt.Sprintf("%v", v.Any()))
	}
	return buf
}

// appendLogText 写入普通文本：仅把 CR/LF/NUL 转义为可见转义序列，
// 防止日志注入（伪造日志行），同时保持既有可读格式。
func appendLogText(buf []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case 0:
			buf = append(buf, '\\', '0')
		default:
			buf = append(buf, s[i])
		}
	}
	return buf
}

// appendStringValue 写入字符串 attr：普通值保持裸文本（兼容既有日志格式），
// 含空白/引号/反斜杠/控制字符时用 Go 引号包裹并转义。
func appendStringValue(buf []byte, s string) []byte {
	if needsQuoting(s) {
		return strconv.AppendQuote(buf, s)
	}
	return append(buf, s...)
}

// needsQuoting 判断字符串值是否需要引号包裹
func needsQuoting(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 0x20 || b == 0x7f || b == ' ' || b == '"' || b == '\\' {
			return true
		}
	}
	return false
}

func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return nil
	}
	c := make([]slog.Attr, len(attrs))
	copy(c, attrs)
	return c
}

// FanoutHandler 多目标分发 Handler
type FanoutHandler struct {
	mu       sync.RWMutex
	handlers map[string]slog.Handler
}

// NewFanoutHandler 创建空的 FanoutHandler
func NewFanoutHandler() *FanoutHandler {
	return &FanoutHandler{
		handlers: make(map[string]slog.Handler),
	}
}

// Add 添加子 handler
func (f *FanoutHandler) Add(name string, h slog.Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[name] = h
}

// Remove 移除子 handler
func (f *FanoutHandler) Remove(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.handlers, name)
}

func (f *FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	f.mu.RLock()
	handlers := make(map[string]slog.Handler, len(f.handlers))
	for k, v := range f.handlers {
		handlers[k] = v
	}
	f.mu.RUnlock()

	var firstErr error
	for _, h := range handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (f *FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	f.mu.RLock()
	defer f.mu.RUnlock()

	nf := NewFanoutHandler()
	for name, h := range f.handlers {
		nf.handlers[name] = h.WithAttrs(attrs)
	}
	return nf
}

func (f *FanoutHandler) WithGroup(name string) slog.Handler {
	f.mu.RLock()
	defer f.mu.RUnlock()

	nf := NewFanoutHandler()
	for n, h := range f.handlers {
		nf.handlers[n] = h.WithGroup(name)
	}
	return nf
}
