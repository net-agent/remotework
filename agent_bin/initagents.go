package main

import (
	"log"
	"sync"

	"github.com/net-agent/remotework/agent"
)

func initAgents(hub *agent.NetHub, agents []agent.AgentInfo) {
	log.Println("startup agents:")

	runcount := 0
	for _, info := range agents {

		mnet := agent.NewNetwork(info)

		var wg sync.WaitGroup
		wg.Add(1)
		go func(network string) {
			ch := make(chan struct{}, 2)
			go mnet.KeepAlive(ch)

			done := false
			for range ch {
				// 重连后，触发hub的网络更新事件
				hub.TriggerNetworkUpdate(network)

				if !done {
					done = true
					wg.Done()
				}
			}
		}(info.Network)
		wg.Wait()

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
