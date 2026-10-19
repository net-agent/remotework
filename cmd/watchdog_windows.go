//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// stillActive 为 GetExitCodeProcess 返回的“进程仍在运行”退出码（259），
// x/sys/windows 未导出该常量
const stillActive = 259

// windowsParentChecker 基于进程句柄等待检测宿主死亡：
// Windows 没有 Unix 的 reparent 语义，父进程退出后 getppid 不会变化，
// 必须打开父进程句柄并 WaitForSingleObject，句柄就绪即宿主已退出。
// 句柄生命周期与进程一致，随进程退出由 OS 回收，无需显式关闭。
type windowsParentChecker struct {
	hostPID int
	handle  windows.Handle
}

// newParentChecker 语义见 watchdog_unix.go。
// OpenProcess(SYNCHRONIZE) 失败时分两类处理：
//  1. 进程已不存在（ERROR_INVALID_PARAMETER）——宿主确实已死，dead=true；
//  2. 权限不足（ERROR_ACCESS_DENIED）——宿主仍存活，只是无权等待它，
//     以 PROCESS_QUERY_LIMITED_INFORMATION + GetExitCodeProcess 区分二者；
//     宿主存活则返回 err（由调用方放弃看门狗，避免误杀健康进程）。
//     若连查询句柄都打不开，说明进程已不可达，按宿主已死处理（防孤儿优先）。
func newParentChecker() (checker parentChecker, dead bool, err error) {
	hostPID := uint32(os.Getppid())

	// SYNCHRONIZE 是等待进程退出所需的最小权限；
	// 托管模式下宿主与本进程同用户同会话，该调用必然成功
	handle, openErr := windows.OpenProcess(windows.SYNCHRONIZE, false, hostPID)
	if openErr == nil {
		return &windowsParentChecker{hostPID: int(hostPID), handle: handle}, false, nil
	}

	alive, queryErr := probeProcessAlive(hostPID)
	switch {
	case queryErr == nil && alive:
		return nil, false, openErr
	case queryErr == nil:
		// 能打开查询句柄但退出码非 STILL_ACTIVE：宿主已死
		return nil, true, openErr
	default:
		// 查询句柄也打不开（进程已消失），按宿主已死处理
		return nil, true, queryErr
	}
}

// probeProcessAlive 用最小查询权限探测进程是否仍存活
func probeProcessAlive(pid uint32) (bool, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(h)

	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false, err
	}
	return code == stillActive, nil
}

func (c *windowsParentChecker) alive() bool {
	// 超时为 0 的等待即检查句柄状态：WAIT_OBJECT_0 表示进程已终止；
	// 等待本身出错（句柄失效等）保守视为宿主已死，防孤儿优先
	event, err := windows.WaitForSingleObject(c.handle, 0)
	if err != nil {
		return false
	}
	return event != windows.WAIT_OBJECT_0
}

func (c *windowsParentChecker) ppid() int {
	return c.hostPID
}
