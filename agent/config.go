package agent

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v2/node"
	"github.com/net-agent/flex/v2/packet"
	"github.com/net-agent/flex/v2/switcher"
	"github.com/net-agent/remotework/utils"
)

type Config struct {
	// Server   ServerInfo    `json:"server"`
	Agent    AgentInfo     `json:"agent"`
	Services []ServiceInfo `json:"services"`
}

type ServerInfo struct {
	Listen   string `json:"listen"`   // 监听的地址
	Password string `json:"password"` // 校验连接的密码
	WsEnable bool   `json:"wsEnable"` // 是否启用Websocket
	WsPath   string `json:"wsPath"`   // Websocket路径
}

type AgentInfo struct {
	Address  string `json:"address"`  // 服务端地址
	Password string `json:"password"` // 连接服务的密码
	Domain   string `json:"domain"`   // 独立域名（不能重复）
	WsEnable bool   `json:"wsEnable"` // 是否为Websocket服务
	Wss      bool   `json:"wss"`      // 是否为wss协议
	WsPath   string `json:"wsPath"`   // Websocket路径
}

type stParam = map[string]string
type ServiceInfo struct {
	Enable bool    `json:"enable"` // 是否启用
	Desc   string  `json:"desc"`   // 描述信息
	Type   string  `json:"type"`   // 类型
	Param  stParam `json:"param"`  // 参数

	index int
}

func (info *ServiceInfo) Name() string {
	return fmt.Sprintf("%v/%v", info.index, info.Type)
}
func (info *ServiceInfo) SetIndex(i int) { info.index = i }

func NewConfig(jsonfile string) (*Config, error) {
	cfg := &Config{}

	err := utils.LoadJSONFile(jsonfile, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *Config) GetConnectFn() ConnectFunc {
	macs, _ := getMacAddr()
	macStr := strings.Join(macs, " ")

	if cfg.Agent.WsEnable {
		return cfg.getWsConnectFn(macStr)
	}

	return cfg.getTcpConnectFn(macStr)
}

func (cfg *Config) getWsConnectFn(mac string) ConnectFunc {

	u := url.URL{
		Scheme: "ws",
		Host:   cfg.Agent.Address,
		Path:   cfg.Agent.WsPath,
	}
	if cfg.Agent.Wss {
		u.Scheme = "wss"
	}
	wsurl := u.String()

	return func() (*node.Node, error) {
		log.Printf("connect to '%v'\n", wsurl)
		c, _, err := websocket.DefaultDialer.Dial(wsurl, nil)
		if err != nil {
			return nil, err
		}
		pc := packet.NewWithWs(c)
		node, err := switcher.UpgradeToNode(
			pc,
			cfg.Agent.Domain,
			mac,
			cfg.Agent.Password,
		)
		if err != nil {
			c.Close()
			return nil, err
		}
		return node, nil
	}
}

func (cfg *Config) getTcpConnectFn(mac string) ConnectFunc {

	return func() (*node.Node, error) {
		log.Printf("connect to '%v'\n", cfg.Agent.Address)
		c, err := net.Dial("tcp4", cfg.Agent.Address)
		if err != nil {
			return nil, err
		}
		pc := packet.NewWithConn(c)
		node, err := switcher.UpgradeToNode(
			pc,
			cfg.Agent.Domain,
			mac,
			cfg.Agent.Password,
		)
		if err != nil {
			c.Close()
			return nil, err
		}
		return node, nil
	}
}

func getMacAddr() ([]string, error) {
	ifas, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var as []string
	for _, ifa := range ifas {
		a := ifa.HardwareAddr.String()
		if a != "" {
			as = append(as, a)
		}
	}
	return as, nil
}
