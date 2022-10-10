package agent

import (
	"fmt"
	"path"
	"strings"

	"github.com/net-agent/remotework/utils"
)

type Config struct {
	AgentMap  map[string]string        `json:"agent" toml:"agent"`
	PipeMap   map[string]PortproxyInfo `json:"pipe" toml:"pipe"`
	SocksMap  map[string]Socks5Info    `json:"sox" toml:"sox"`
	Agents    []AgentInfo              `json:"agents" toml:"agents"`
	Portproxy []PortproxyInfo          `json:"portproxy" toml:"portproxy"`
	Socks5    []Socks5Info             `json:"socks5" toml:"socks5"`
	RDP       []RDPInfo                `json:"rdp" toml:"rdp"`
	Visit     []QuickVisitInfo         `json:"visit" toml:"visit"`
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
	return cfg, err
}

type ServerInfo struct {
	Listen   string `json:"listen" toml:"listen"`     // 监听的地址
	Password string `json:"password" toml:"password"` // 校验连接的密码
	WsEnable bool   `json:"wsEnable" toml:"wsEnable"` // 是否启用Websocket
	WsPath   string `json:"wsPath" toml:"wsPath"`     // Websocket路径
}

type AgentInfo struct {
	Enable     bool   `json:"enable" toml:"enable"`
	Name       string `json:"name" toml:"name"` // 网络名称，不能为tcp、tcp4、tcp6
	Protocol   string `json:"protocol" toml:"protocol"`
	Address    string `json:"address" toml:"address"`   // 服务端地址
	Password   string `json:"password" toml:"password"` // 连接服务的密码
	Domain     string `json:"domain" toml:"domain"`     // 独立域名（不能重复）
	URL        string `json:"url" toml:"url"`           // <Network>://<Domain>:<Password>@<Address>
	WsEnable   bool   `json:"wsEnable" toml:"wsEnable"` // 是否为Websocket服务
	Wss        bool   `json:"wss" toml:"wss"`           // 是否为wss协议
	WsPath     string `json:"wsPath" toml:"wsPath"`     // Websocket路径
	QuickTrust Trust  `json:"trust" toml:"trust"`
}

type Trust struct {
	Enable    bool              `json:"enable" toml:"enable"`
	WhiteList map[string]string `json:"whiteList" toml:"whiteList"`
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

type QuickVisitInfo struct {
	ListenURL string `json:"listen" toml:"listen"`
	TargetURL string `json:"target" toml:"target"`
	LogName   string `json:"log" toml:"log"`
}

type RDPInfo struct {
	ListenURL string `json:"listen" toml:"listen"`
	LogName   string `json:"log" toml:"log"`
}
