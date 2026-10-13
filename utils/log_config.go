package utils

import (
	"fmt"
	"log/slog"
	"os"
)

var (
	globalFanout *FanoutHandler
	globalLevel  slog.LevelVar
)

func init() {
	globalFanout = NewFanoutHandler()
	globalFanout.Add("console", NewConsoleHandler(os.Stderr, &globalLevel))
}

// GlobalHandler 返回全局 FanoutHandler
func GlobalHandler() *FanoutHandler {
	return globalFanout
}

// SetLevel 动态调整全局日志级别
func SetLevel(level slog.Level) {
	globalLevel.Set(level)
}

// SetFormat 切换控制台输出格式
// format: "text"（默认，人类可读，写 stderr）或 "json"（slog JSON，写 stdout）。
// json 模式供 Host 托管运行（sidecar 协议 C3 通道）：stdout 承载结构化日志，
// stderr 仅保留 panic 等运行库输出。
func SetFormat(format string) {
	switch format {
	case "json":
		globalFanout.Add("console", slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: &globalLevel,
		}))
	default:
		globalFanout.Add("console", NewConsoleHandler(os.Stderr, &globalLevel))
	}
}

// NewModuleLogger 创建带模块名的 *slog.Logger（使用全局 Handler）
func NewModuleLogger(module string) *slog.Logger {
	return slog.New(GlobalHandler()).With("module", module)
}

// Fatal 输出 Error 日志后退出程序
func Fatal(log *slog.Logger, v ...any) {
	log.Error(fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf 格式化版本的 Fatal
func Fatalf(log *slog.Logger, format string, v ...any) {
	log.Error(fmt.Sprintf(format, v...))
	os.Exit(1)
}
