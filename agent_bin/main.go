package main

import (
	"log"
	"sync"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/agent/service"
)

func loadConfig() *agent.Config {
	var flags agent.AgentFlags
	flags.Parse()

	// 读取配置
	log.Printf("> read config from '%v'\n", flags.ConfigFileName)
	var err error
	config, err := agent.NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	return config
}

func main() {
	config := loadConfig()

	mnet := agent.NewNetwork(nil)
	ch := make(chan struct{}, 4)
	go mnet.KeepAlive(ch)

	// 初始化services
	svcs := []service.Service{}
	for _, info := range config.Services {
		svc := service.NewService(mnet, info)
		if svc != nil {
			svcs = append(svcs, svc)
		} else {
			log.Printf("unknown service type: %v\n", info.Type)
		}
	}

	// 开启服务
	log.Println("startup services:")
	log.Println("-------------------------------------------------------------------------")
	log.Println("  # command        type                   listen                   target")
	log.Println("-------------------------------------------------------------------------")

	var wg sync.WaitGroup
	for i, svc := range svcs {
		wg.Add(1)
		go func(index int, svc service.Service) {
			svc.Run()
			wg.Done()
			log.Printf("service stopped, index=%v. %v\n", index, svc.Info())
		}(i, svc)
		log.Printf("%3v %7v %v\n", i, "run", svc.Info())
	}
	log.Println("-------------------------------------------------------------------------")

	wg.Wait()
}
