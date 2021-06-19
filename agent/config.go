package main

import (
	"fmt"
	"io"
	"sync"

	"github.com/net-agent/flex"
	"github.com/net-agent/remotework/utils"
)

type Config struct {
	Server   ServerInfo    `json:"tunnel"`
	Services []ServiceInfo `json:"services"`
}

type ServerInfo struct {
	Address string `json:"address"`
	Vhost   string `json:"vhost"`
}

type ServiceInfo struct {
	Enable bool              `json:"enable"`
	Desc   string            `json:"description"`
	Type   string            `json:"type"`
	Param  map[string]string `json:"param"`

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

func (info *ServiceInfo) Run(wg *sync.WaitGroup, host *flex.Host) error {
	defer wg.Done()
	if !info.Enable {
		return nil
	}

	switch info.Type {

	case "socks5":
		return nil

	case "portproxy":
		svc := NewPortproxy(host, info.Param["target"])
		l, err := listen(host, info.Param["listen"])
		if err != nil {
			return err
		}
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
