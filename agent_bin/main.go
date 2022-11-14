package main

import (
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewNamedLogger("sys", false)

func main() {
	config := loadConfig()

	hub := agent.NewHub()
	hub.MountConfig(config)

	go waitCloseSignal(hub)

	initSysTray(hub)
	defer releaseSysTray()

	hub.StartServices()
	syslog.Println("main process exit.")
}
