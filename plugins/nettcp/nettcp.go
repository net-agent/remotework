// Package nettcp 内置本地 TCP 网络插件（tcp/tcp4/tcp6）。
package nettcp

import (
	"context"

	"github.com/net-agent/flex/v3/stream"
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
	bus, err := kernel.Inject[kernel.EventBus](c, "")
	if err != nil {
		return err
	}

	alias := p.alias
	trace := &vnet.StreamTrace{
		OnStreamOpen: func(st *stream.State) {
			bus.Publish(vnet.TopicStreamOpen, vnet.StreamEvent{Network: alias, State: st})
		},
		OnStreamClosed: func(st *stream.State) {
			bus.Publish(vnet.TopicStreamClosed, vnet.StreamEvent{Network: alias, State: st})
		},
	}
	network := vnet.NewReportingNetwork(vnet.NewTcpNetwork(alias, trace))
	// 插件卸载/进程关停时关闭追踪表中的活跃连接，让 connTracker.CloseAll("stopped") 真正执行
	if err := c.DeferCleanup(func(context.Context) error {
		network.Stop()
		return nil
	}); err != nil {
		return err
	}
	return kernel.Provide[vnet.Network](c, alias, network)
}

func (p *Plugin) Start(c *kernel.Context) error { return nil }
