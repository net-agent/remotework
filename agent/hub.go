package agent

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/net-agent/remotework/utils"
)

type Hub struct {
	log      *slog.Logger
	networks *NetworkRegistry
	services *ServiceManager
	stopOnce sync.Once
	done     chan struct{}
	running  int32 // atomic: 0=stopped, 1=running

	listenerMu sync.RWMutex
	listeners  []HubEventListener
}

func NewHub(log *slog.Logger) *Hub {
	if log == nil {
		log = utils.NewModuleLogger("hub")
	}
	hub := &Hub{
		log:      log,
		networks: NewNetworkRegistry(log.With("module", "hub.net")),
		services: NewServiceManager(log.With("module", "hub.svc")),
		done:     make(chan struct{}),
	}
	hub.services.onStatusChange = func(name string, oldStatus, newStatus ServiceStatus) {
		hub.notifyServiceStatusChange(name, oldStatus, newStatus)
	}
	return hub
}

func (hub *Hub) MountConfig(cfg *Config) error {
	var errs []string

	for _, info := range cfg.Agents {

		if err := hub.NewAgentNetwork(info); err != nil {
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

// Start 阻塞式启动所有服务，等待 Stop() 被调用后清理退出
func (hub *Hub) Start() error {
	atomic.StoreInt32(&hub.running, 1)
	go hub.services.StartAll()
	<-hub.done
	atomic.StoreInt32(&hub.running, 0)
	hub.networks.StopAll()
	return nil
}

// Stop 幂等优雅关闭，触发所有服务退出并解除 Start() 阻塞
func (hub *Hub) Stop() {
	hub.stopOnce.Do(func() {
		hub.log.Info("stopping hub...")
		hub.services.StopAll()
		close(hub.done)
	})
}

// NewAgentNetwork 创建并注册一个 agent 网络，消除外部对 NewNetwork 的直接依赖
func (hub *Hub) NewAgentNetwork(info AgentInfo) error {
	vnet, err := NewNetwork(hub, info)
	if err != nil {
		return err
	}
	return hub.networks.Add(vnet)
}

// Dial 供 NetworkRegistry 使用
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

func (hub *Hub) AddNetwork(mnet Network) error               { return hub.networks.Add(mnet) }
func (hub *Hub) FindNetwork(network string) (Network, error) { return hub.networks.Find(network) }
func (hub *Hub) AddService(svc *Service) error               { return hub.services.Add(svc) }
func (hub *Hub) AddAndStartService(svc *Service) error {
	if err := hub.services.Add(svc); err != nil {
		return err
	}
	hub.services.Start(svc)
	return nil
}
func (hub *Hub) FindService(name string) (*Service, error) { return hub.services.Find(name) }
func (hub *Hub) IsRunning() bool                           { return atomic.LoadInt32(&hub.running) == 1 }
func (hub *Hub) RangeAllService(fn func(svc *Service))     { hub.services.Range(fn) }

//
// Deprecated 方法，保持向后兼容
//

// Deprecated: Use Start()
func (hub *Hub) StartServices() error { return hub.Start() }

// Deprecated: Use Stop()
func (hub *Hub) StopServices() { hub.Stop() }

// Deprecated: Stop() 已包含网络清理
func (hub *Hub) StopNetworks() { hub.networks.StopAll() }

//
// 事件监听器管理
//

func (hub *Hub) AddEventListener(l HubEventListener) {
	hub.listenerMu.Lock()
	defer hub.listenerMu.Unlock()
	hub.listeners = append(hub.listeners, l)
}

func (hub *Hub) RemoveEventListener(l HubEventListener) {
	hub.listenerMu.Lock()
	defer hub.listenerMu.Unlock()
	for i, li := range hub.listeners {
		if li == l {
			hub.listeners = append(hub.listeners[:i], hub.listeners[i+1:]...)
			return
		}
	}
}

// NotifyNetworkStateChange 实现 NetworkStateNotifier 接口
func (hub *Hub) NotifyNetworkStateChange(name, oldState, newState string) {
	hub.notifyNetworkStateChange(name, oldState, newState)
}

func (hub *Hub) notifyNetworkStateChange(name, oldState, newState string) {
	hub.listenerMu.RLock()
	snapshot := make([]HubEventListener, len(hub.listeners))
	copy(snapshot, hub.listeners)
	hub.listenerMu.RUnlock()

	for _, l := range snapshot {
		l := l
		go l.OnNetworkStateChange(name, oldState, newState)
	}
}

func (hub *Hub) notifyServiceStatusChange(name string, oldStatus, newStatus ServiceStatus) {
	hub.listenerMu.RLock()
	snapshot := make([]HubEventListener, len(hub.listeners))
	copy(snapshot, hub.listeners)
	hub.listenerMu.RUnlock()

	for _, l := range snapshot {
		l := l
		go l.OnServiceStatusChange(name, oldStatus, newStatus)
	}
}
