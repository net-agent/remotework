package utils

import (
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
// format: "text"（默认）或 "json"
func SetFormat(format string) {
	switch format {
	case "json":
		globalFanout.Add("console", slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: &globalLevel,
		}))
	default:
		globalFanout.Add("console", NewConsoleHandler(os.Stderr, &globalLevel))
	}
}
