package agent

import (
	"net"
	"strings"

	"github.com/net-agent/remotework/utils"
)

type Hub struct {
	nl       *utils.NamedLogger
	Networks *NetworkRegistry
	Services *ServiceManager
}

func NewHub() *Hub {
	return &Hub{
		nl:       utils.NewNamedLogger("hub", false),
		Networks: NewNetworkRegistry(),
		Services: NewServiceManager(),
	}
}

func (hub *Hub) MountConfig(cfg *Config) {
	var err error

	for _, info := range cfg.Agents {
		err = hub.Networks.Add(NewNetwork(hub, hub, info))
		if err != nil {
			hub.nl.Printf("network register failed. err='%v'\n", err)
		}
	}
	for _, info := range cfg.Portproxy {
		err = hub.Services.Add(NewPortproxyService(hub.Networks, hub.Networks, info))
		if err != nil {
			hub.nl.Printf("service register failed. err='%v'\n", err)
		}
	}
	for _, info := range cfg.Socks5 {
		err = hub.Services.Add(NewSocks5Service(hub.Networks, info))
		if err != nil {
			hub.nl.Printf("service register failed. err='%v'\n", err)
		}
	}
	for _, info := range cfg.RDP {
		err = hub.Services.Add(NewRDPService(hub.Networks, hub.Networks, info))
		if err != nil {
			hub.nl.Printf("service register failed. err='%v'\n", err)
		}
	}

	// load config summary
	hub.nl.Printf("registered networks: %v\n", strings.Join(hub.Networks.Names(), ", "))
	hub.nl.Printf("registered services: %v\n", strings.Join(hub.Services.Names(), ", "))
}

// UpdateNetwork 实现 NetworkUpdateNotifier 接口，桥接 Networks 和 Services
func (hub *Hub) UpdateNetwork(network string) {
	hub.Services.UpdateByNetwork(network)
}

// Dial 实现 RawDialer 接口，供 NewNetwork 使用
func (hub *Hub) Dial(network, addr string) (net.Conn, error) {
	return hub.Networks.Dial(network, addr)
}

// ListenURL 实现 ListenerFactory 接口（委托）
func (hub *Hub) ListenURL(raw string) (net.Listener, error) {
	return hub.Networks.ListenURL(raw)
}

// URLDialer 实现 DialerFactory 接口（委托）
func (hub *Hub) URLDialer(raw string) (QuickDialer, error) {
	return hub.Networks.URLDialer(raw)
}

//
// 以下为外部 API 兼容的委托方法
//

func (hub *Hub) AddNetwork(mnet Network) error        { return hub.Networks.Add(mnet) }
func (hub *Hub) FindNetwork(network string) (Network, error) { return hub.Networks.Find(network) }
func (hub *Hub) AddService(svc *Service) error         { return hub.Services.Add(svc) }
func (hub *Hub) FindService(name string) (*Service, error) { return hub.Services.Find(name) }
func (hub *Hub) StartServices() error                  { return hub.Services.StartAll() }
func (hub *Hub) StopServices()                         { hub.Services.StopAll() }
func (hub *Hub) StopNetworks()                         { hub.Networks.StopAll() }
func (hub *Hub) IsRunning() bool                       { return hub.Services.IsRunning() }
func (hub *Hub) RangeAllService(fn func(svc *Service)) { hub.Services.Range(fn) }
