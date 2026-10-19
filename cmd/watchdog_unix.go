//go:build !windows

package main

import "os"

// unixParentChecker 基于 ppid 变化检测宿主死亡：
// Unix 下父进程退出后子进程会被 init/launchd 收养，getppid 随之变化。
// 检测无需特权系统调用，一次 getppid 的开销可忽略。
type unixParentChecker struct {
	hostPID int
}

// newParentChecker 返回检测器；dead 为 true 表示启动时宿主已确认死亡，
// err 非 nil（且 dead 为 false）表示宿主存活但无法建立监测。
// Unix 上 getppid 不会失败，后两种情况不会发生。
func newParentChecker() (checker parentChecker, dead bool, err error) {
	return &unixParentChecker{hostPID: os.Getppid()}, false, nil
}

func (c *unixParentChecker) alive() bool {
	return os.Getppid() == c.hostPID
}

func (c *unixParentChecker) ppid() int {
	return c.hostPID
}
