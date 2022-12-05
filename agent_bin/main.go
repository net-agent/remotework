package main

import (
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewNamedLogger("sys", false)

func main() {
	var flags ClientFlags
	flags.Parse()

	// 处理ping命令
	if flags.PingDomain != "" {
		handlePingDomain(flags.PingDomain, flags.PingClientName)
		return
	}

	config := loadConfig(&flags)

	hub := agent.NewHub()
	hub.MountConfig(config)

	go waitCloseSignal(hub)

	initSysTray(hub)
	defer releaseSysTray()

	hub.StartServices()
	syslog.Println("main process exit.")
}
