package utils

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Logger 基于 slog 的模块化日志器
type Logger struct {
	sl   *slog.Logger
	name string
}

// NewLogger 创建指定模块名的 Logger
func NewLogger(module string) *Logger {
	module = strings.TrimSpace(module)
	if module == "" {
		// 空模块名返回一个不输出的 logger
		return &Logger{}
	}
	return &Logger{
		sl:   slog.New(GlobalHandler()).With("module", module),
		name: module,
	}
}

// Debug 输出 Debug 级别日志
func (l *Logger) Debug(msg string, args ...any) {
	if l.sl == nil {
		return
	}
	l.sl.Debug(msg, args...)
}

// Info 输出 Info 级别日志
func (l *Logger) Info(msg string, args ...any) {
	if l.sl == nil {
		return
	}
	l.sl.Info(msg, args...)
}

// Warn 输出 Warn 级别日志
func (l *Logger) Warn(msg string, args ...any) {
	if l.sl == nil {
		return
	}
	l.sl.Warn(msg, args...)
}

// Error 输出 Error 级别日志
func (l *Logger) Error(msg string, args ...any) {
	if l.sl == nil {
		return
	}
	l.sl.Error(msg, args...)
}

// Printf 兼容旧风格，映射到 Info 级别
func (l *Logger) Printf(format string, v ...any) {
	if l.sl == nil {
		return
	}
	msg := fmt.Sprintf(format, v...)
	msg = strings.TrimRight(msg, "\n")
	l.sl.Info(msg)
}

// Println 兼容旧风格，映射到 Info 级别
func (l *Logger) Println(v ...any) {
	if l.sl == nil {
		return
	}
	msg := fmt.Sprintln(v...)
	msg = strings.TrimRight(msg, "\n")
	l.sl.Info(msg)
}

// Fatal 输出 Error 级别日志后退出程序
func (l *Logger) Fatal(v ...any) {
	if l.sl != nil {
		msg := fmt.Sprint(v...)
		l.sl.Error(msg)
	}
	os.Exit(1)
}

// Name 返回模块名
func (l *Logger) Name() string {
	return l.name
}
