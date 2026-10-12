// Package nettcp 内置本地 TCP 网络插件（tcp/tcp4/tcp6）。
package nettcp

import (
	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/vnet"
)

// Plugin 本地 TCP 网络插件
type Plugin struct {
	alias string
}

// New 创建内置网络插件，alias 为 "tcp" / "tcp4" / "tcp6"
func New(alias string) *Plugin {
	return &Plugin{alias: alias}
}

func (p *Plugin) Info() kernel.Info {
	return kernel.Info{Name: "network/" + p.alias, Kind: "network"}
}

func (p *Plugin) Init(c *kernel.Context) error {
	return kernel.Provide[vnet.Network](c, p.alias,
		vnet.NewReportingNetwork(vnet.NewTcpNetwork(p.alias)))
}

func (p *Plugin) Start(c *kernel.Context) error { return nil }
