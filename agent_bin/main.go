package main

import (
	"log"

	"github.com/getlantern/systray"
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
	defer systray.Quit()

	log.Println("net hub is working, click systray to get more infos.")
	hub.Wait()
	log.Println("main process exit.")
}
