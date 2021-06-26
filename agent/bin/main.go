package main

import (
	"log"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/agent/netx"
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
	ev := make(chan string, 10)
	go netx.KeepHostAlive(config, ev)

	svcs := []service.Service{}
	for _, info := range config.Services {
		svcs = append(svcs, service.NewService(info))
	}

	for event := range ev {

		switch event {

		case netx.EvConnected:
			// 开启服务
			log.Println("startup services:")
			log.Println("-------------------------------------------------------------------------")
			log.Println("  # command        type                   listen                   target")
			log.Println("-------------------------------------------------------------------------")
			for i, svc := range svcs {
				go svc.Run()
				log.Printf("%3v %7v %v\n", i, "run", svc.Info())
			}
			log.Println("-------------------------------------------------------------------------")

		case netx.EvDisconnected:
			// 关闭服务
			log.Println("shutdown services:")
			log.Println("-------------------------------------------------------------------------")
			log.Println("  # command        type                   listen                   target")
			log.Println("-------------------------------------------------------------------------")
			for i, svc := range svcs {
				go svc.Close()
				log.Printf("%5v %5v %v\n", i, "close", svc.Info())
			}
			log.Println("-------------------------------------------------------------------------")

		default:
			log.Printf("unknown events: '%v'\n", event)
		}

	}
}
