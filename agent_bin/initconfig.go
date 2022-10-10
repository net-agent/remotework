package main

import (
	"encoding/json"
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

	// parse agents url
	for i := 0; i < len(config.Agents); i++ {
		if config.Agents[i].URL != "" {
			u, err := url.Parse(config.Agents[i].URL)
			var ok bool
			if err == nil {
				config.Agents[i].Name = u.Scheme
				config.Agents[i].Protocol = "tcp" // 默认只通过tcp连接服务端
				config.Agents[i].Domain = u.User.Username()
				config.Agents[i].Password, ok = u.User.Password()
				if !ok {
					config.Agents[i].Password = ""
				}
				config.Agents[i].Address = u.Host
			}
		}
	}

	// parse agent name map
	// 与agents数组的url类似，但是url里的scheme含义发生了变化
	agentMap := make(map[string]string)
	json.Unmarshal(config.AgentMap, &agentMap)
	for k, v := range agentMap {
		u, err := url.Parse(v)
		if err == nil {
			var ok bool
			var ag agent.AgentInfo
			ag.Name = k
			ag.Protocol = u.Scheme
			ag.Domain = u.User.Username()
			ag.Password, ok = u.User.Password()
			ag.WsPath = u.Path
			if !ok {
				ag.Password = ""
			}
			ag.Address = u.Host

			config.Agents = append(config.Agents, ag)
		}
	}

	// parse pipe map
	// 是portproxy的别名，简化书写
	pipeMap := make(map[string]agent.PortproxyInfo)
	json.Unmarshal(config.PipeMap, &pipeMap)
	for k, v := range pipeMap {
		v.LogName = k
		config.Portproxy = append(config.Portproxy, v)
	}

	// porse sox
	// 是socks5的别名，简化书写
	socksMap := make(map[string]agent.Socks5Info)
	json.Unmarshal(config.SocksMap, &socksMap)
	for k, v := range socksMap {
		v.LogName = k
		config.Socks5 = append(config.Socks5, v)
	}

	return config
}
