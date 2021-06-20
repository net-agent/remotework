package main

import (
	"log"

	"github.com/net-agent/flex"
)

func main() {
	var flags ServerFlags
	flags.Parse()

	// 读取配置
	log.Printf("read config from '%v'\n", flags.ConfigFileName)
	config, err := NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	// 初始化
	sw := flex.NewSwitcher(nil, config.Server.Password)

	log.Printf("try to listen on '%v'\n", config.Server.Listen)
	sw.Run(config.Server.Listen)
}
