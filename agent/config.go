package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/net-agent/remotework/utils"
)

type Config struct {
	AgentMap  json.RawMessage `json:"agent" toml:"agent"`
	PipeMap   json.RawMessage `json:"pipe" toml:"pipe"`
	SocksMap  json.RawMessage `json:"sox" toml:"sox"`
	Agents    []AgentInfo     `json:"agents" toml:"agents"`
	Portproxy []PortproxyInfo `json:"portproxy" toml:"portproxy"`
	Socks5    []Socks5Info    `json:"socks5" toml:"socks5"`
	RDP       []RDPInfo       `json:"rdp" toml:"rdp"`
}
type ServerInfo struct {
	Listen   string `json:"listen" toml:"listen"`     // 监听的地址
	Password string `json:"password" toml:"password"` // 校验连接的密码
	WsEnable bool   `json:"wsEnable" toml:"wsEnable"` // 是否启用Websocket
	WsPath   string `json:"wsPath" toml:"wsPath"`     // Websocket路径
}

type AgentInfo struct {
	Enable   bool   `json:"enable" toml:"enable"`
	Name     string `json:"name" toml:"name"` // 网络名称，不能为tcp、tcp4、tcp6
	Protocol string `json:"protocol" toml:"protocol"`
	Address  string `json:"address" toml:"address"`   // 服务端地址
	Password string `json:"password" toml:"password"` // 连接服务的密码
	Domain   string `json:"domain" toml:"domain"`     // 独立域名（不能重复）
	URL      string `json:"url" toml:"url"`           // <Network>://<Domain>:<Password>@<Address>
	WsEnable bool   `json:"wsEnable" toml:"wsEnable"` // 是否为Websocket服务
	Wss      bool   `json:"wss" toml:"wss"`           // 是否为wss协议
	WsPath   string `json:"wsPath" toml:"wsPath"`     // Websocket路径
}

type PortproxyInfo struct {
	ListenURL string `json:"listen" toml:"listen"`
	TargetURL string `json:"target" toml:"target"`
	LogName   string `json:"log" toml:"log"`
}

type Socks5Info struct {
	ListenURL string `json:"listen" toml:"listen"`
	Username  string `json:"username" toml:"username"`
	Password  string `json:"password" toml:"password"`
	LogName   string `json:"log" toml:"password"`
}

type RDPInfo struct {
	ListenURL string `json:"listen" toml:"listen"`
	LogName   string `json:"log" toml:"log"`
}

func NewConfig(configFileName string) (*Config, error) {
	cfg := &Config{}
	var err error
	switch strings.ToLower(path.Ext(configFileName)) {
	case ".json":
		err = utils.LoadJSONFile(configFileName, cfg)
	case ".toml":
		err = utils.LoadTomlFile(configFileName, cfg)
	default:
		err = fmt.Errorf("config file [%s] not support, must be json or toml", configFileName)
	}
	cfg.PreProcess()
	return cfg, err
}

func (config *Config) PreProcess() {
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
			var ag AgentInfo
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
	config.AgentMap = nil

	// parse pipe map
	// 是portproxy的别名，简化书写
	pipeMap := make(map[string]PortproxyInfo)
	json.Unmarshal(config.PipeMap, &pipeMap)
	for k, v := range pipeMap {
		v.LogName = k
		config.Portproxy = append(config.Portproxy, v)
	}
	config.PipeMap = nil

	// porse sox
	// 是socks5的别名，简化书写
	socksMap := make(map[string]Socks5Info)
	json.Unmarshal(config.SocksMap, &socksMap)
	for k, v := range socksMap {
		v.LogName = k
		config.Socks5 = append(config.Socks5, v)
	}
	config.SocksMap = nil
}
