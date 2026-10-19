package main

import (
	"log/slog"
	"time"
)

// parentCheckInterval 宿主存活检测周期
const parentCheckInterval = 2 * time.Second

// parentChecker 平台相关的父进程（宿主）存活检测器，
// 实现见 watchdog_unix.go / watchdog_windows.go
type parentChecker interface {
	// alive 报告启动时记录的宿主进程是否仍存活
	alive() bool
	// ppid 返回启动时记录的宿主进程 PID，仅用于日志
	ppid() int
}

// startParentWatchdog 仅托管模式（managedMode，-log-format=json）下启动
// 父进程存活看门狗（R3）：桌面端（Tauri）以 sidecar 方式拉起本进程，
// 宿主被 SIGKILL 时没有任何清理机会，本进程会被 init/launchd 收养成为
// 孤儿并长期占用端口。检测到宿主死亡后走与 POST /api/v1/stop 相同的
// 优雅退出路径（requestStop），不直接 os.Exit。
//
// standalone（终端启动、text 日志）绝不启用：用户关闭终端后 ppid
// 同样会变化，不能误杀。
func startParentWatchdog(requestStop func(), log *slog.Logger) {
	if !managedMode {
		return
	}

	checker, dead, err := newParentChecker()
	switch {
	case dead:
		// 启动时宿主已不存在（如宿主拉起本进程后立刻崩溃），直接触发优雅退出
		log.Warn("host process already gone at startup, shutting down", "err", err)
		requestStop()
		return
	case err != nil:
		// 宿主确认存活但无法建立监测（如权限不足）：放弃看门狗，
		// 不误杀健康进程，代价是宿主下次异常退出时本进程可能孤儿化
		log.Warn("parent watchdog disabled: cannot monitor host process", "err", err)
		return
	}

	log.Info("parent watchdog started", "ppid", checker.ppid(), "interval", parentCheckInterval)

	// 进程退出路径无需专门停本 goroutine（进程将死），但 ticker 要 Stop
	go func() {
		ticker := time.NewTicker(parentCheckInterval)
		defer ticker.Stop()

		for range ticker.C {
			if checker.alive() {
				continue
			}
			log.Warn("host process exited, shutting down", "ppid", checker.ppid())
			requestStop()
			return
		}
	}()
}
