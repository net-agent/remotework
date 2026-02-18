package agent

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/net-agent/remotework/utils"
)

type Hub struct {
	log      *slog.Logger
	networks *NetworkRegistry
	services *ServiceManager
	stopOnce sync.Once
}

func NewHub(log *slog.Logger) *Hub {
	if log == nil {
		log = utils.NewModuleLogger("hub")
	}
	return &Hub{
		log:      log,
		networks: NewNetworkRegistry(log.With("module", "hub.net")),
		services: NewServiceManager(log.With("module", "hub.svc")),
	}
}

func (hub *Hub) MountConfig(cfg *Config) error {
	var errs []string

	for _, info := range cfg.Agents {
		if err := hub.networks.Add(NewNetwork(hub, hub, info, hub.log)); err != nil {
			hub.log.Warn("network register failed", "err", err)
			errs = append(errs, fmt.Sprintf("network: %v", err))
		}
	}
	for _, info := range cfg.Portproxy {
		if err := hub.services.Add(NewPortproxyService(hub.networks, hub.networks, info)); err != nil {
			hub.log.Warn("service register failed", "err", err)
			errs = append(errs, fmt.Sprintf("portproxy: %v", err))
		}
	}
	for _, info := range cfg.Socks5 {
		if err := hub.services.Add(NewSocks5Service(hub.networks, info)); err != nil {
			hub.log.Warn("service register failed", "err", err)
			errs = append(errs, fmt.Sprintf("socks5: %v", err))
		}
	}
	for _, info := range cfg.RDP {
		if err := hub.services.Add(NewRDPService(hub.networks, hub.networks, info)); err != nil {
			hub.log.Warn("service register failed", "err", err)
			errs = append(errs, fmt.Sprintf("rdp: %v", err))
		}
	}

	// load config summary
	hub.log.Info("registered networks", "names", strings.Join(hub.networks.Names(), ", "))
	hub.log.Info("registered services", "names", strings.Join(hub.services.Names(), ", "))

	if len(errs) > 0 {
		return fmt.Errorf("mount config had %d error(s): %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// Start 阻塞式启动所有服务，返回后自动清理网络连接
func (hub *Hub) Start() error {
	err := hub.services.StartAll()
	hub.networks.StopAll() // 所有服务退出后，清理网络
	return err
}

// Stop 幂等优雅关闭，触发所有服务退出
func (hub *Hub) Stop() {
	hub.stopOnce.Do(func() {
		hub.log.Info("stopping hub...")
		hub.services.StopAll()
	})
}

// NewAgentNetwork 创建并注册一个 agent 网络，消除外部对 NewNetwork 的直接依赖
func (hub *Hub) NewAgentNetwork(info AgentInfo) error {
	return hub.networks.Add(NewNetwork(hub, hub, info, hub.log))
}

// UpdateNetwork 实现 NetworkUpdateNotifier 接口，桥接 Networks 和 Services
func (hub *Hub) UpdateNetwork(network string) {
	hub.services.UpdateByNetwork(network)
}

// Dial 实现 RawDialer 接口，供 NewNetwork 使用
func (hub *Hub) Dial(network, addr string) (net.Conn, error) {
	return hub.networks.Dial(network, addr)
}

// ListenURL 实现 ListenerFactory 接口（委托）
func (hub *Hub) ListenURL(raw string) (net.Listener, error) {
	return hub.networks.ListenURL(raw)
}

// URLDialer 实现 DialerFactory 接口（委托）
func (hub *Hub) URLDialer(raw string) (QuickDialer, error) {
	return hub.networks.URLDialer(raw)
}

//
// 外部 API 兼容的委托方法
//

func (hub *Hub) AddNetwork(mnet Network) error              { return hub.networks.Add(mnet) }
func (hub *Hub) FindNetwork(network string) (Network, error) { return hub.networks.Find(network) }
func (hub *Hub) AddService(svc *Service) error               { return hub.services.Add(svc) }
func (hub *Hub) FindService(name string) (*Service, error)   { return hub.services.Find(name) }
func (hub *Hub) IsRunning() bool                             { return hub.services.IsRunning() }
func (hub *Hub) RangeAllService(fn func(svc *Service))       { hub.services.Range(fn) }

//
// Deprecated 方法，保持向后兼容
//

// Deprecated: Use Start()
func (hub *Hub) StartServices() error { return hub.Start() }

// Deprecated: Use Stop()
func (hub *Hub) StopServices() { hub.Stop() }

// Deprecated: Stop() 已包含网络清理
func (hub *Hub) StopNetworks() { hub.networks.StopAll() }
