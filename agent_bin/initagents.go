package main

import (
	"github.com/net-agent/remotework/agent"
)

func initAgents(hub *agent.Hub, agents []agent.AgentInfo) {
	syslog.Println("register agents:")

	runcount := 0
	for _, info := range agents {
		mnet := agent.NewNetwork(hub, info)
		err := hub.AddNetwork(info.Name, mnet)
		if err != nil {
			syslog.Printf("register agent failed. name='%v', err=%v\n", info.Name, err)
			continue
		}
		runcount++
	}
	if runcount == 0 {
		syslog.Println("WARN: NO AGENT REGISTERED")
	}
}
