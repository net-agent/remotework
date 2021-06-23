package main

import (
	"fmt"
	"io"
	"sync"

	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/socks"
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

func (info *ServiceInfo) Run(wg *sync.WaitGroup) error {
	defer wg.Done()
	if !info.Enable {
		return nil
	}

	switch info.Type {

	case "socks5":
		l, err := listen(info.Param["listen"])
		if err != nil {
			return err
		}

		username := info.Param["username"]
		password := info.Param["password"]
		svc := socks.NewPswdServer(username, password)
		info.closer = svc
		return svc.Run(l)

	case "portproxy":
		l, err := listen(info.Param["listen"])
		if err != nil {
			return err
		}

		svc := NewPortproxy(info.Param["target"])
		info.closer = svc
		return svc.Run(l)
	}

	return nil
}

func (info *ServiceInfo) Info() string {
	switch info.Type {
	case "socks5":
		return info.Param["listen"]
	case "portproxy":
		return fmt.Sprintf("%v > %v", info.Param["listen"], info.Param["target"])
	default:
		return "unknown svc"
	}
}
