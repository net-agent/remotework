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

	initSysTray(hub)
	defer releaseSysTray()

	// 打印状态
	syslog.Println(hub.GetAllNetworkString())
	syslog.Println(hub.GetAllServiceStateString())

	hub.StartServices()
	syslog.Println("main process exit.")
}
