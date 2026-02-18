package main

import (
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

func loadConfig(flags *ClientFlags) *agent.Config {
	configName, err := utils.ResolveConfigFile(flags.ConfigFileName)
	if err != nil {
		utils.Fatal(syslog, "load config failed: ", err)
	}
	syslog.Info("read config", "path", configName)
	config, err := agent.NewConfig(configName)
	if err != nil {
		utils.Fatal(syslog, "load config failed: ", err)
	}

	return config
}
