package main

import (
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/server"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewNamedLogger("sys", false)

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
		syslog.Fatal("invalid run-mode:", flags.RunMode)
	}
}

func RunServiceMode(flags *ClientFlags) {
	config := loadConfig(flags)

	hub := agent.NewHub()
	hub.MountConfig(config)
	initSysTray(hub)
	defer releaseSysTray()

	go waitCloseSignal(hub)
	hub.StartServices()
	syslog.Println("main process exit.")
}

func RunCLIMode(flags *ClientFlags) {
	handlePingDomain(flags.PingDomain, flags.PingClientName, 8)
}
