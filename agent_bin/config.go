package main

import (
	"path"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

func loadConfig() *agent.Config {
	var flags ClientFlags
	flags.Parse()

	// 读取配置
	configName := flags.ConfigFileName
	if !utils.FileExist(configName) {
		syslog.Printf("load '%v' failed, try config.json/config.toml\n", configName)
		// try `config.json` or `config.toml`
		dir := path.Dir(configName)
		configJson := path.Join(dir, "config.json")
		configToml := path.Join(dir, "config.toml")
		if utils.FileExist(configJson) {
			configName = configJson
		} else if utils.FileExist(configToml) {
			configName = configToml
		} else {
			syslog.Fatal("load config failed: config file not exist!")
		}
	}
	syslog.Printf("read config from '%v'\n", configName)
	config, err := agent.NewConfig(configName)
	if err != nil {
		syslog.Fatal("load config failed: ", err)
	}

	return config
}
