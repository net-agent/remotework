package main

import (
	"log"
	"sync"
)

var config *Config

func initConfig() {
	var flags AgentFlags
	flags.Parse()

	// 读取配置
	log.Printf("> read config from '%v'\n", flags.ConfigFileName)
	var err error
	config, err = NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}
}

func main() {
	initConfig()
	_, err := getHost()
	if err != nil {
		log.Fatal("get host failed: ", err)
	}

	var wg sync.WaitGroup

	// 开启服务
	log.Println("-------------------------------------------------------------------------")
	log.Println("state index        type                   listen                   target")
	log.Println("-------------------------------------------------------------------------")

	for i := 0; i < len(config.Services); i++ {
		svc := config.Services[i]

		enable := "stop"
		if svc.Enable {
			enable = "run"
		}

		log.Printf("%5v %5v %11v %24v %24v\n", enable, i, svc.Type, svc.Param["listen"], svc.Param["target"])
		wg.Add(1)
		go func(svc *ServiceInfo) {
			svc.Run(&wg)
		}(&svc)
	}

	log.Println("-------------------------------------------------------------------------")

	wg.Wait()
	log.Println("agent stopped")
}
