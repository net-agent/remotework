package main

import (
	"log"
	"net/url"
	"path"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

func loadConfig() *agent.Config {
	var flags agent.AgentFlags
	flags.Parse()

	// 读取配置
	configName := flags.ConfigFileName
	if !utils.FileExist(configName) {
		// try `config.json` or `config.toml`
		dir := path.Dir(configName)
		configJson := path.Join(dir, "config.json")
		configToml := path.Join(dir, "config.toml")
		if utils.FileExist(configJson) {
			configName = configJson
		} else if utils.FileExist(configToml) {
			configName = configToml
		} else {
			log.Fatal("load config failed: config file not exist!")
		}
	}
	log.Printf("read config from '%v'\n", configName)
	config, err := agent.NewConfig(configName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	// parse agents url
	for i := 0; i < len(config.Agents); i++ {
		if config.Agents[i].URL != "" {
			u, err := url.Parse(config.Agents[i].URL)
			var ok bool
			if err == nil {
				config.Agents[i].Network = u.Scheme
				config.Agents[i].Domain = u.User.Username()
				config.Agents[i].Password, ok = u.User.Password()
				if !ok {
					config.Agents[i].Password = ""
				}
				config.Agents[i].Address = u.Host
			}
		}
	}

	return config
}
