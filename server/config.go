package server

import (
	"fmt"
	"net"
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

// Validate 校验 relay 配置，避免空口令/空监听等“默认不安全”配置静默启动。
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	listen := strings.TrimSpace(c.Server.Listen)
	if listen == "" {
		return fmt.Errorf("server.listen is required")
	}
	if _, _, err := net.SplitHostPort(listen); err != nil {
		return fmt.Errorf("server.listen %q is invalid: %w", listen, err)
	}
	if strings.TrimSpace(c.Server.Password) == "" {
		return fmt.Errorf("server.password is required (empty password allows anyone to join the relay)")
	}
	if len(c.Server.Password) < 8 {
		return fmt.Errorf("server.password is too short (minimum 8 characters)")
	}
	if c.Server.WsEnable {
		if c.Server.WsPath == "" {
			return fmt.Errorf("server.wsPath is required when wsEnable=true")
		}
		if !strings.HasPrefix(c.Server.WsPath, "/") {
			return fmt.Errorf("server.wsPath must start with '/': %q", c.Server.WsPath)
		}
	}
	return nil
}

func NewConfig(configFile string) (*Config, error) {
	cfg := &Config{}
	if err := utils.LoadConfigFile(configFile, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
