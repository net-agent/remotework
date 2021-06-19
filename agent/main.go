package main

import (
	"log"
	"net"
	"sync"

	"github.com/net-agent/flex"
)

func main() {
	var flags AgentFlags
	flags.Parse()

	// 读取配置
	config, err := NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed:", err)
	}

	// 创建连接
	conn, err := net.Dial("tcp4", config.Server.Address)
	if err != nil {
		log.Fatal("dial failed:", err)
	}

	// 协议转换
	host, err := flex.UpgradeToHost(conn, &flex.HostRequest{
		Domain: config.Server.Vhost,
		Mac:    "test-mac-token",
	})
	if err != nil {
		log.Fatal("upgrade failed:", err)
	}

	log.Printf("server connected: %v\n", config.Server.Address)

	var wg sync.WaitGroup

	// 开启服务
	log.Println("-------------------------------------------------------------------------")
	log.Println("state index        type                   listen                   target")
	log.Println("-------------------------------------------------------------------------")
	for i, svc := range config.Services {
		enable := "stop"
		if svc.Enable {
			enable = "run"
		}

		log.Printf("%5v %5v %11v %24v %24v\n", enable, i, svc.Type, svc.Param["listen"], svc.Param["target"])
		wg.Add(1)
		go svc.Run(&wg, host)
	}
	log.Println("-------------------------------------------------------------------------")

	wg.Wait()
}
