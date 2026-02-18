package utils

import (
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
