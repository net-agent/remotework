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

	log.Println("server connected")

	var wg sync.WaitGroup

	// 开启服务
	log.Println("---------------------------------------")
	for i, svc := range config.Services {
		log.Printf("%2v %2v %9v %20v\n", i, "x", svc.Type, svc.Desc)
		wg.Add(1)
		go svc.Run(&wg, host)
	}
	log.Println("---------------------------------------")

	wg.Wait()
}
