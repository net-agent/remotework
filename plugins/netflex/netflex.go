// Package netflex flex 虚拟网络插件：每个 link 对应一个插件实例。
package netflex

import (
	"context"
	"fmt"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/remotework/vnet"
)

// TopicState 网络会话状态变化的 EventBus 主题，payload 为 StateEvent
const TopicState = "network.state"

// 流生命周期事件的主题与类型定义在 vnet 包（TopicStreamOpen/TopicStreamClosed、
// StreamEvent），tcp 与 flex 网络共用同一事件管道。

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

	auth, err := resolveAuth(p.link.Auth, p.link.AuthRef)
	if err != nil {
		return err
	}

	alias := p.alias
	mnet, err := vnet.NewFlexNetworkFromLink(vnet.FlexLinkConfig{
		Alias:    alias,
		Scheme:   p.link.Scheme,
		RelayURL: p.link.RelayURL,
		Domain:   p.link.As,
		Auth:     auth,
		Logger:   c.Logger(),
		OnStateChange: func(oldState, newState string) {
			bus.Publish(TopicState, StateEvent{Network: alias, Old: oldState, New: newState})
		},
		OnStreamOpen: func(st *stream.State) {
			bus.Publish(vnet.TopicStreamOpen, vnet.StreamEvent{Network: alias, State: st})
		},
		OnStreamClosed: func(st *stream.State) {
			bus.Publish(vnet.TopicStreamClosed, vnet.StreamEvent{Network: alias, State: st})
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

// resolveAuth 解析 link 的认证密钥。
//
// authRef 存在时优先使用它，并通过 utils.ResolveSecret 解析（env:NAME 或裸
// 环境变量名）。解析失败必须返回错误、让插件 Failed（fail closed）；绝不能
// 把 authRef 的字面量当作密钥使用。
func resolveAuth(auth, authRef string) (string, error) {
	if authRef == "" {
		return auth, nil
	}
	secret, err := utils.ResolveSecret(authRef)
	if err != nil {
		return "", fmt.Errorf("resolve link authRef: %w", err)
	}
	return secret, nil
}
