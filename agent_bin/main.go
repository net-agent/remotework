package main

import (
	"os"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewNamedLogger("sys", false)

func main() {
	config := loadConfig()

	// 初始化日志文件
	logoutput, shouldClose := initLogOutput()
	if shouldClose && logoutput != nil {
		defer logoutput.Close()
	}

	hub := agent.NewHub()
	initAgents(hub, config.Agents)
	initServices(hub, config)
	initSysTray(hub)
	defer releaseSysTray()

	// 打印状态
	hub.NetworkReportAscii(os.Stdout)
	hub.ServiceReportAscii(os.Stdout)

	hub.StartServices()
	syslog.Println("main process exit.")
}
