package main

import (
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

func loadConfig(flags *ClientFlags) *agent.Config {
	configName, err := utils.ResolveConfigFile(flags.ConfigFileName)
	if err != nil {
		syslog.Fatal("load config failed: ", err)
	}
	syslog.Printf("read config from '%v'\n", configName)
	config, err := agent.NewConfig(configName)
	if err != nil {
		syslog.Fatal("load config failed: ", err)
	}

	return config
}
