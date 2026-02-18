package utils

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// NamedLogger 兼容层，内部直接包装 *slog.Logger
// 新代码应直接使用 *slog.Logger，此类型仅为过渡保留
type NamedLogger struct {
	Log *slog.Logger // 导出，方便外部直接使用
}

// NewNamedLogger 创建 NamedLogger（asyncOutput 参数保留但不再生效）
func NewNamedLogger(name string, asyncOutput bool) *NamedLogger {
	name = strings.TrimSpace(name)
	if name == "" {
		return &NamedLogger{Log: slog.New(GlobalHandler())}
	}
	return &NamedLogger{Log: NewModuleLogger(name)}
}

// Printf 兼容旧风格，映射到 Info 级别
func (nl *NamedLogger) Printf(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	msg = strings.TrimRight(msg, "\n")
	nl.Log.Info(msg)
}

// Println 兼容旧风格，映射到 Info 级别
func (nl *NamedLogger) Println(v ...any) {
	msg := fmt.Sprintln(v...)
	msg = strings.TrimRight(msg, "\n")
	nl.Log.Info(msg)
}

// Fatal 输出 Error 级别日志后退出程序
func (nl *NamedLogger) Fatal(v ...any) {
	nl.Log.Error(fmt.Sprint(v...))
	os.Exit(1)
}

// Close 保留为空操作（兼容旧接口）
func (nl *NamedLogger) Close() {}

// Deprecated: 全局输出目标由 Handler 层管理，此函数不再生效
func SetNamedLoggerOutputDist(dist *os.File) {}
