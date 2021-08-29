package main

import (
	"log"

	"github.com/net-agent/remotework/agent"
)

func initAgents(hub *agent.NetHub, agents []agent.AgentInfo) {
	log.Println("startup agents:")

	runcount := 0
	for _, info := range agents {

		mnet := agent.NewNetwork(info.GetConnectFn())
		ch := make(chan struct{}, 2)
		go mnet.KeepAlive(ch)
		<-ch

		// if info.QuickTrust.Enable {
		// 	trust := service.NewQuickT(hub, info.Network, info.QuickTrust.WhiteList)
		// 	trust.Init()
		// 	trust.Start()
		// }

		err := hub.AddNetwork(info.Network, mnet)
		if err != nil {
			log.Printf("add network failed. network='%v', err=%v\n", info.Network, err)
			continue
		}

		runcount++
	}
	if runcount == 0 {
		log.Println("WARN: NO AGENTS ARE RUNNING")
	}
}
