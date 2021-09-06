package main

import (
	"log"
	"os"

	"github.com/net-agent/remotework/agent"
)

func main() {
	config := loadConfig()

	// 初始化日志文件
	logoutput, shouldClose := initLogOutput()
	if shouldClose && logoutput != nil {
		defer logoutput.Close()
	}

	hub := agent.NewNetHub()
	initAgents(hub, config.Agents)
	initServices(hub, config)
	initSysTray(hub)
	defer releaseSysTray()

	// 打印状态
	hub.NetworkReportAscii(os.Stdout)
	hub.ServiceReportAscii(os.Stdout)

	hub.Wait()
	log.Println("main process exit.")
}
