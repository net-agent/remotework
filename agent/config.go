package agent

import (
	"io"

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

	closer io.Closer
}

func NewConfig(jsonfile string) (*Config, error) {
	cfg := &Config{}

	err := utils.LoadJSONFile(jsonfile, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
