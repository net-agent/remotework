package utils

import (
	"os"
)

// NamedLogger 兼容层，内部委托给 Logger
type NamedLogger struct {
	*Logger
}

// NewNamedLogger 创建 NamedLogger（asyncOutput 参数保留但不再生效）
func NewNamedLogger(name string, asyncOutput bool) *NamedLogger {
	return &NamedLogger{Logger: NewLogger(name)}
}

// Close 保留为空操作（兼容旧接口）
func (nl *NamedLogger) Close() {}

// Deprecated: 全局输出目标由 Handler 层管理，此函数不再生效
func SetNamedLoggerOutputDist(dist *os.File) {}
