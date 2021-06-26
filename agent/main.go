package main

import (
	"log"
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

	ev := make(chan string, 10)
	go keepHostAlive(config, ev)

	for event := range ev {

		log.Println("-------------------------------------------------------------------------")
		log.Println("state index        type                   listen                   target")
		log.Println("-------------------------------------------------------------------------")
		switch event {

		case evConnected:
			// 开启服务
			for i := 0; i < len(config.Services); i++ {
				svc := config.Services[i]

				enable := "stop"
				if svc.Enable {
					enable = "run"
				}

				log.Printf("%5v %5v %11v %24v %24v\n", enable, i, svc.Type, svc.Param["listen"], svc.Param["target"])
				go svc.Run()
			}

		case evDisconnected:
			// 开启服务
			for i := 0; i < len(config.Services); i++ {
				svc := config.Services[i]

				enable := "stop"

				log.Printf("%5v %5v %11v %24v %24v\n", enable, i, svc.Type, svc.Param["listen"], svc.Param["target"])
				if svc.closer != nil {
					go svc.closer.Close()
				}
			}

		default:
			log.Printf("unknown events: '%v'\n", event)
		}

		log.Println("-------------------------------------------------------------------------")
	}
}
