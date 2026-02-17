package server

import (
	"fmt"
	"path"
	"strings"

	"github.com/net-agent/remotework/utils"
)

type Config struct {
	Server ServerInfo `json:"server" toml:"server"`
}

type ServerInfo struct {
	Listen   string `json:"listen" toml:"listen"`     // 监听的地址
	Password string `json:"password" toml:"password"` // 校验连接的密码
	WsEnable bool   `json:"wsEnable" toml:"wsEnable"` // 是否启用Websocket
	WsPath   string `json:"wsPath" toml:"wsPath"`     // Websocket路径
}

func NewConfig(configFile string) (*Config, error) {
	cfg := &Config{}
	var err error
	switch strings.ToLower(path.Ext(configFile)) {
	case ".json":
		err = utils.LoadJSONFile(configFile, cfg)
	case ".toml":
		err = utils.LoadTomlFile(configFile, cfg)
	default:
		err = fmt.Errorf("config file [%s] not supported, must be json or toml", configFile)
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
