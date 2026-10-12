// Package netflex flex 虚拟网络插件：每个 link 对应一个插件实例。
package netflex

import (
	"context"

	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/vnet"
)

// TopicState 网络会话状态变化的 EventBus 主题，payload 为 StateEvent
const TopicState = "network.state"

// StateEvent 网络会话状态变化事件
type StateEvent struct {
	Network string // 网络别名
	Old     string // 旧状态（Ready/Idle/Connecting/Online/Closed）
	New     string // 新状态
}

// Plugin flex 虚拟网络插件
type Plugin struct {
	alias string
	link  configv2.LinkSpec
}

// New 由 link 配置创建网络插件
func New(alias string, link configv2.LinkSpec) *Plugin {
	return &Plugin{alias: alias, link: link}
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
	mnet, err := vnet.NewFlexNetworkFromLink(vnet.FlexLinkConfig{
		Alias:    alias,
		Scheme:   p.link.Scheme,
		RelayURL: p.link.RelayURL,
		Domain:   p.link.As,
		Auth:     resolveAuth(p.link.Auth, p.link.AuthRef),
		Logger:   c.Logger(),
		OnStateChange: func(oldState, newState string) {
			bus.Publish(TopicState, StateEvent{Network: alias, Old: oldState, New: newState})
		},
	})
	if err != nil {
		return err
	}
	// session 已在创建时启动 Serve；Init 失败回滚或插件卸载时关闭
	if err := c.DeferCleanup(func(ctx context.Context) error { mnet.Stop(); return nil }); err != nil {
		mnet.Stop()
		return err
	}
	return kernel.Provide[vnet.Network](c, alias, vnet.NewReportingNetwork(mnet))
}

func (p *Plugin) Start(c *kernel.Context) error { return nil }

// resolveAuth 返回认证密钥，authRef 优先（当前直接返回引用名，待 secret provider 落地）
func resolveAuth(auth, authRef string) string {
	if authRef != "" {
		return authRef
	}
	return auth
}
