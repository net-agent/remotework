package agent

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"time"

	"github.com/net-agent/remotework/agent/configv2"
	"github.com/net-agent/remotework/agent/vnet"
	"github.com/net-agent/remotework/agent/vservice"
	"github.com/net-agent/remotework/utils"
)

// HubEventListener 可选的事件监听器，用于接收 Hub 内部状态变化通知
type HubEventListener interface {
	OnNetworkStateChange(name, oldState, newState string)
	OnServiceStatusChange(name string, oldStatus, newStatus vservice.ServiceStatus)
}

// NetworkStateNotifier 网络状态变化通知接口
type NetworkStateNotifier interface {
	NotifyNetworkStateChange(name, oldState, newState string)
}

type Hub struct {
	log        *slog.Logger
	networks   *NetworkRegistry
	registry   *vservice.ServiceRegistry
	supervisor *vservice.ServiceSupervisor
	stopOnce   sync.Once
	done       chan struct{}
	running    int32 // atomic: 0=stopped, 1=running

	listenerMu sync.RWMutex
	listeners  []HubEventListener
}

func NewHub(log *slog.Logger) *Hub {
	if log == nil {
		log = utils.NewModuleLogger("hub")
	}
	svcLog := log.With("module", "hub.svc")
	registry := vservice.NewServiceRegistry(svcLog)
	supervisor := vservice.NewServiceSupervisor(svcLog, registry)
	hub := &Hub{
		log:        log,
		networks:   NewNetworkRegistry(log.With("module", "hub.net")),
		registry:   registry,
		supervisor: supervisor,
		done:       make(chan struct{}),
	}
	supervisor.OnStatusChange = func(name string, oldStatus, newStatus vservice.ServiceStatus) {
		hub.notifyServiceStatusChange(name, oldStatus, newStatus)
	}
	supervisor.IsDependencyErr = func(err error) bool {
		var depErr *vnet.ErrDependencyNotReady
		return errors.As(err, &depErr)
	}
	return hub
}

// MountConfig loads a CanonicalConfig (already normalized and validated) into the hub.
func (hub *Hub) MountConfig(cfg *CanonicalConfig) error {
	var errs []string

	// 注册 link 网络
	for alias, link := range cfg.Links {
		if err := hub.mountLink(alias, link); err != nil {
			hub.log.Warn("link mount failed", "alias", alias, "err", err)
			errs = append(errs, fmt.Sprintf("link %s: %v", alias, err))
		}
	}

	// 注册 tunnel 服务
	for i, tunnel := range cfg.Tunnels {
		if err := hub.mountTunnel(tunnel); err != nil {
			hub.log.Warn("tunnel mount failed", "index", i, "name", tunnel.Name, "err", err)
			errs = append(errs, fmt.Sprintf("tunnel %s: %v", tunnel.Name, err))
		}
	}

	// load config summary
	hub.log.Info("registered networks", "names", strings.Join(hub.networks.Names(), ", "))
	hub.log.Info("registered services", "names", strings.Join(hub.registry.Names(), ", "))

	if len(errs) > 0 {
		return fmt.Errorf("mount config had %d error(s): %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// mountLink creates a flex network from a link spec and registers it.
func (hub *Hub) mountLink(alias string, link configv2.LinkSpec) error {
	n, err := vnet.NewFlexNetworkFromLink(vnet.FlexLinkConfig{
		Alias:    alias,
		Scheme:   link.Scheme,
		RelayURL: link.RelayURL,
		Domain:   link.As,
		Auth:     resolveAuth(link.Auth, link.AuthRef),
		OnStateChange: func(oldState, newState string) {
			hub.NotifyNetworkStateChange(alias, oldState, newState)
		},
	})
	if err != nil {
		return err
	}
	return hub.networks.Add(n)
}

// resolveAuth returns the auth value, preferring authRef (environment variable) over plain auth.
func resolveAuth(auth, authRef string) string {
	if authRef != "" {
		// TODO: resolve from environment variable or secret manager
		// For now, return authRef as-is (will be implemented properly)
		return authRef
	}
	return auth
}

// mountTunnel creates a service from a tunnel spec and registers it.
func (hub *Hub) mountTunnel(tunnel configv2.TunnelSpec) error {
	// Each endpoint carries its own authcode via URL query.
	// endpointToInternalURL injects ?authcode= for the registry's cipherconn handling.
	listenURL := endpointToInternalURL(tunnel.Listen)
	targetURL := endpointToInternalURL(tunnel.Target)

	var svc *vservice.Service

	switch tunnel.Target.Kind {
	case configv2.EndpointBuiltin:
		switch tunnel.Target.Scheme {
		case "socks5":
			svc = vservice.NewSocks5Service(
				hub.networks, tunnel.Name, listenURL,
				tunnel.Target.Username, tunnel.Target.Password,
			)
		default:
			return fmt.Errorf("unsupported builtin service: %s", tunnel.Target.Scheme)
		}
	default:
		svc = vservice.NewPortproxyService(
			hub.networks, hub.networks,
			tunnel.Name, listenURL, targetURL,
		)
	}

	if svc == nil {
		return fmt.Errorf("failed to create service for tunnel %s", tunnel.Name)
	}

	// Store tunnel ID in service for tracking
	svc.TunnelID = tunnel.ID

	return hub.registry.Add(svc)
}

// resolveAuthcode returns the authcode value, preferring authcodeRef over plain authcode.
func resolveAuthcode(code, codeRef string) string {
	if codeRef != "" {
		// TODO: resolve from environment variable or secret manager
		return codeRef
	}
	return code
}

// endpointToInternalURL converts an EndpointSpec to the internal URL format
// that NetworkRegistry understands. For vtcp endpoints, authcode from the
// EndpointSpec is appended as ?authcode= for cipherconn transport encryption.
//
// Mapping:
//   - tcp://host:port            -> tcp://host:port (unchanged)
//   - vtcp://vhost.alias:port    -> alias://vhost:port[?authcode=xxx]
//   - socks5://...               -> socks5://... (unchanged)
func endpointToInternalURL(ep configv2.EndpointSpec) string {
	switch ep.Kind {
	case configv2.EndpointVNet:
		// vtcp://db.office:3306 -> office://db:3306[?authcode=xxx]
		u := fmt.Sprintf("%s://%s:%s", ep.Alias, ep.VHostname, ep.Port)
		authcode := resolveAuthcode(ep.Authcode, ep.AuthcodeRef)
		if authcode != "" {
			u += "?authcode=" + url.QueryEscape(authcode)
		}
		return u
	case configv2.EndpointBuiltin:
		return ep.RawURL
	default:
		// tcp://host:port -> tcp://host:port
		return ep.RawURL
	}
}

// Start 阻塞式启动所有服务，等待 Stop() 被调用后清理退出
func (hub *Hub) Start() error {
	atomic.StoreInt32(&hub.running, 1)
	go hub.supervisor.StartAll()
	<-hub.done
	atomic.StoreInt32(&hub.running, 0)
	hub.networks.StopAll()
	return nil
}

// MountLink creates and registers a link network at runtime (for API use).
func (hub *Hub) MountLink(alias string, link configv2.LinkSpec) error {
	return hub.mountLink(alias, link)
}

// MountTunnel creates and starts a tunnel service at runtime (for API use).
func (hub *Hub) MountTunnel(tunnel configv2.TunnelSpec) error {
	if err := hub.mountTunnel(tunnel); err != nil {
		return err
	}
	svc, err := hub.registry.Find(tunnel.Name)
	if err != nil {
		return err
	}
	hub.supervisor.Start(svc)
	return nil
}

// Stop 幂等优雅关闭，触发所有服务退出并解除 Start() 阻塞
func (hub *Hub) Stop() {
	hub.stopOnce.Do(func() {
		hub.log.Info("stopping hub...")
		hub.supervisor.StopAll()
		close(hub.done)
	})
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
func (hub *Hub) AddService(svc *Service) error               { return hub.registry.Add(svc) }
func (hub *Hub) AddAndStartService(svc *Service) error {
	if err := hub.registry.Add(svc); err != nil {
		return err
	}
	hub.supervisor.Start(svc)
	return nil
}
func (hub *Hub) FindService(name string) (*Service, error) { return hub.registry.Find(name) }
func (hub *Hub) IsRunning() bool                           { return atomic.LoadInt32(&hub.running) == 1 }
func (hub *Hub) RangeAllService(fn func(svc *Service))     { hub.registry.Range(fn) }

//
// 状态查询委托方法
//

func (hub *Hub) GetAllServiceState() ([]ServiceState, error)  { return hub.registry.GetAllState() }
func (hub *Hub) GetAllServiceStateString() string             { return hub.registry.GetAllStateString() }
func (hub *Hub) GetAllNetworkState() ([]NetworkReport, error) { return hub.networks.GetAllState() }
func (hub *Hub) GetAllNetworkStateString() string             { return hub.networks.GetAllStateString() }
func (hub *Hub) GetAllDataStreamStateString() string {
	return hub.networks.GetAllDataStreamStateString()
}
func (hub *Hub) GetDataStreamState(limits int, networks ...string) []*DataStreamState {
	return hub.networks.GetDataStreamState(limits, networks...)
}
func (hub *Hub) PingDomain(network, domain string) (time.Duration, error) {
	return hub.networks.PingDomain(network, domain)
}

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
