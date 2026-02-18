package main

import (
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/server"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewModuleLogger("sys")

func main() {
	var flags ClientFlags
	flags.Parse()

	switch flags.RunMode {
	case "agent":
		RunServiceMode(&flags)
	case "server":
		server.RunServer(flags.ConfigFileName)
	case "cli":
		RunCLIMode(&flags)
	default:
		utils.Fatal(syslog, "invalid run-mode: ", flags.RunMode)
	}
}

func RunServiceMode(flags *ClientFlags) {
	config := loadConfig(flags)

	// 启动 pprof 服务器
	var pprofServer *utils.PprofServer
	if config.Pprof.Enable {
		pprofServer = utils.NewPprofServer(syslog.With("module", "pprof"))
		pprofServer.Start(config.Pprof.Listen)
		defer pprofServer.Stop()
	}

	hub := agent.NewHub(nil)
	if err := hub.MountConfig(config); err != nil {
		syslog.Warn("mount config warning", "err", err)
	}
	initSysTray(hub)
	defer releaseSysTray()

	go waitCloseSignal(hub)
	hub.Start()
	syslog.Info("main process exit")
}

func RunCLIMode(flags *ClientFlags) {
	handlePingDomain(flags.PingDomain, flags.PingClientName, 8)
}
