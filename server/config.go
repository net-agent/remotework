package server

import (
	"github.com/net-agent/remotework/utils"
)

type Config struct {
	Server ServerInfo `json:"server" toml:"server"`
	// Agent    AgentInfo     `json:"agent"`
	// Services []ServiceInfo `json:"services"`
}

type ServerInfo struct {
	Listen   string `json:"listen" toml:"listen"`     // 监听的地址
	Password string `json:"password" toml:"password"` // 校验连接的密码
	WsEnable bool   `json:"wsEnable" toml:"wsEnable"` // 是否启用Websocket
	WsPath   string `json:"wsPath" toml:"wsPath"`     // Websocket路径
}

func NewConfig(jsonfile string) (*Config, error) {
	cfg := &Config{}

	err := utils.LoadJSONFile(jsonfile, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
