// Package apiplugin 把 api 控制面包装为 kernel 观察器插件。
// 通过 Catalog/EventBus 两个内核预置能力驱动 api.Backend。
package apiplugin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/api"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/plugins/netflex"
	"github.com/net-agent/remotework/plugins/tunnel"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// 插件命名约定（与 plugins/doc.go 一致）
const (
	networkNamePrefix = "network/"
	tunnelNamePrefix  = "tunnel/"
)

// Plugin api 观察器插件
type Plugin struct {
	cfg     configv2.APIInfo
	onStop  func()
	backend *backend
	server  *api.Server
}

// New 创建 api 插件；onStop 为请求停止宿主的回调（由 cmd 注入）
func New(cfg configv2.APIInfo, onStop func()) *Plugin {
	return &Plugin{cfg: cfg, onStop: onStop}
}

func (p *Plugin) Info() kernel.Info {
	return kernel.Info{Name: "api", Kind: "observer"}
}

func (p *Plugin) Init(c *kernel.Context) error {
	catalog, err := kernel.Inject[kernel.Catalog](c, "")
	if err != nil {
		return err
	}
	bus, err := kernel.Inject[kernel.EventBus](c, "")
	if err != nil {
		return err
	}

	p.backend = &backend{
		ctx:     c,
		catalog: catalog,
		bus:     bus,
		onStop:  p.onStop,
		log:     c.Logger(),
	}
	p.backend.running.Store(true)
	p.server = api.New(p.backend, p.cfg, c.Logger())
	c.DeferCleanup(func(ctx context.Context) error {
		p.backend.running.Store(false)
		p.server.Stop()
		return nil
	})
	return nil
}

func (p *Plugin) Start(c *kernel.Context) error {
	return p.server.Start()
}

//
// api.Backend 实现
//

type backend struct {
	ctx     *kernel.Context
	catalog kernel.Catalog
	bus     kernel.EventBus
	onStop  func()
	log     *slog.Logger
	running atomic.Bool
}

func (b *backend) IsRunning() bool { return b.running.Load() }

func (b *backend) GetAllNetworkState() ([]vnet.NetworkReport, error) {
	var reports []vnet.NetworkReport
	for _, ps := range b.catalog.List() {
		if ps.Info.Kind != "network" {
			continue
		}
		alias := strings.TrimPrefix(ps.Info.Name, networkNamePrefix)
		rn, ok := kernel.Lookup[vnet.Network](b.ctx, alias)
		if !ok {
			continue
		}
		if rp, ok := rn.(interface{ Report() vnet.NetworkReport }); ok {
			reports = append(reports, rp.Report())
		}
	}
	return reports, nil
}

func (b *backend) GetAllServiceState() ([]vservice.ServiceState, error) {
	var states []vservice.ServiceState
	for _, ps := range b.catalog.List() {
		if ps.Info.Kind != "tunnel" {
			continue
		}
		name := strings.TrimPrefix(ps.Info.Name, tunnelNamePrefix)
		svc, ok := kernel.Lookup[*vservice.Service](b.ctx, name)
		if !ok {
			continue
		}
		states = append(states, svc.ServiceState)
	}
	return states, nil
}

// streamStateProvider 用于获取数据流状态的可选接口
type streamStateProvider interface {
	GetStreamStates() (actives, closeds []*stream.State)
}

// streamStatesOf 穿透 ReportingNetwork 装饰器读取流状态
func streamStatesOf(mnet vnet.Network) (actives, closeds []*stream.State) {
	inner := mnet
	if rn, ok := mnet.(*vnet.ReportingNetwork); ok {
		inner = rn.Network
	}
	provider, ok := inner.(streamStateProvider)
	if !ok {
		return nil, nil
	}
	return provider.GetStreamStates()
}

func (b *backend) GetDataStreamState(limit int, networks ...string) []*api.DataStreamState {
	resp := []*api.DataStreamState{}
	for _, network := range networks {
		mnet, ok := kernel.Lookup[vnet.Network](b.ctx, network)
		if !ok {
			resp = append(resp, nil)
			continue
		}
		actives, closeds := streamStatesOf(mnet)
		size := len(closeds)
		if size > limit {
			closeds = closeds[size-limit : size]
		}
		resp = append(resp, &api.DataStreamState{Network: network, Actives: actives, Closeds: closeds})
	}
	return resp
}

func (b *backend) PingDomain(network, domain string) (time.Duration, error) {
	mnet, ok := kernel.Lookup[vnet.Network](b.ctx, network)
	if !ok {
		return 0, fmt.Errorf("network='%v' not found", network)
	}
	return mnet.Ping(domain, time.Second*3)
}

func (b *backend) MountLink(alias string, link configv2.LinkSpec) error {
	p := netflex.New(alias, link)
	if err := b.catalog.Load(p); err != nil {
		return err
	}
	return b.waitSettled(p.Info().Name)
}

func (b *backend) MountTunnel(spec configv2.TunnelSpec) error {
	p := tunnel.New(spec)
	if err := b.catalog.Load(p); err != nil {
		return err
	}
	return b.waitSettled(p.Info().Name)
}

// waitSettled 等待插件进入稳定态：Failed 时卸载并返回错误；
// Running/Pending 视为成功（Pending 会在依赖就绪后自动启动）
func (b *backend) waitSettled(name string) error {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, err := b.catalog.State(name)
		if err != nil {
			return err
		}
		switch st.State {
		case kernel.StateRunning, kernel.StatePending:
			return nil
		case kernel.StateFailed:
			b.catalog.Unload(name, kernel.UnloadOptions{})
			if st.LastErr != "" {
				return errors.New(st.LastErr)
			}
			return errors.New("plugin init failed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil // 超时：仍在初始化，交由异步推进
}

func (b *backend) RequestStop() {
	if b.running.CompareAndSwap(true, false) && b.onStop != nil {
		go b.onStop()
	}
}

func (b *backend) SubscribeNetworkState(buf int) (<-chan api.NetworkStateEvent, func()) {
	src, unsub := b.bus.SubscribeTopic(buf, netflex.TopicState)
	out := make(chan api.NetworkStateEvent, buf)
	go func() {
		defer close(out)
		for evt := range src {
			if se, ok := evt.Payload.(netflex.StateEvent); ok {
				out <- api.NetworkStateEvent{
					Network:  se.Network,
					OldState: se.Old,
					NewState: se.New,
				}
			}
		}
	}()
	return out, unsub
}

func (b *backend) SubscribeServiceStatus(buf int) (<-chan api.ServiceStatusEvent, func()) {
	src, unsub := b.catalog.Subscribe(buf)
	out := make(chan api.ServiceStatusEvent, buf)
	go func() {
		defer close(out)
		for evt := range src {
			if !strings.HasPrefix(evt.Plugin, tunnelNamePrefix) {
				continue
			}
			name := strings.TrimPrefix(evt.Plugin, tunnelNamePrefix)
			switch evt.Kind {
			case kernel.EventStateChanged:
				out <- api.ServiceStatusEvent{
					Name:      name,
					OldStatus: serviceStatusOf(evt.From),
					NewStatus: serviceStatusOf(evt.To),
				}
			case kernel.EventUnloaded:
				out <- api.ServiceStatusEvent{
					Name:      name,
					OldStatus: serviceStatusOf(evt.From),
					NewStatus: "stopped",
				}
			}
		}
	}()
	return out, unsub
}

// serviceStatusOf 将插件生命周期状态映射为旧版服务状态文案，保持 UI 兼容
func serviceStatusOf(s kernel.State) string {
	switch s {
	case kernel.StateRunning:
		return "running"
	case kernel.StatePending:
		return "pending"
	case kernel.StateFailed:
		return "init failed"
	case kernel.StateInitializing, kernel.StateReady, kernel.StateStarting:
		return "init"
	case kernel.StateStopping, kernel.StateUnloaded:
		return "stopped"
	default:
		return "uninit"
	}
}
