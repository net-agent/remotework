package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/remotework/utils"
)

type Hub struct {
	nl   *utils.NamedLogger
	nets map[string]Network
	mut  sync.RWMutex

	svcs     []*Service
	svcNames map[string]*Service
	svcMut   sync.RWMutex
	svcID    int32
}

func NewHub() *Hub {
	hub := &Hub{
		nl:       utils.NewNamedLogger("hub", false),
		nets:     make(map[string]Network),
		svcNames: make(map[string]*Service),
	}

	hub.AddNetwork(newTcpNetwork("tcp"))
	hub.AddNetwork(newTcpNetwork("tcp4"))
	hub.AddNetwork(newTcpNetwork("tcp6"))

	return hub
}

func (hub *Hub) MountConfig(cfg *Config) {
	cfg.PreProcess()

	for _, info := range cfg.Agents {
		hub.AddNetwork(NewNetwork(hub, info))
	}
	for _, info := range cfg.Portproxy {
		hub.AddService(NewPortproxyService(hub, info))
	}
	for _, info := range cfg.Socks5 {
		hub.AddService(NewSocks5Service(hub, info))
	}
	for _, info := range cfg.RDP {
		hub.AddService(NewRDPService(hub, info))
	}
}

func (hub *Hub) UpdateNetwork(network string) {
	count := 0
	for _, svc := range hub.svcs {
		if svc.IsDepend(network) {
			go svc.controller.Update()
			count++
		}
	}
	hub.nl.Printf("update network='%v', %v service updated\n", network, count)
}

func (hub *Hub) AddService(svc *Service) error {
	svc.ID = atomic.AddInt32(&hub.svcID, 1)

	hub.svcMut.Lock()
	defer hub.svcMut.Unlock()

	if _, found := hub.svcNames[svc.Name]; found {
		hub.nl.Printf("service register failed. dump service name='%v'\n", svc.Name)
		return errors.New("dump service name")
	}

	svc.State = "uninit"
	hub.svcs = append(hub.svcs, svc)
	hub.svcNames[svc.Name] = svc
	hub.nl.Printf("service registered. name='%v'\n", svc.Name)

	return nil
}

func (hub *Hub) FindService(name string) (*Service, error) {
	hub.svcMut.RLock()
	defer hub.svcMut.RUnlock()

	svc, found := hub.svcNames[name]
	if !found {
		return nil, errors.New("service not found")
	}
	return svc, nil
}

func (hub *Hub) StartServices() {
	hub.nl.Println("start services:")
	var wg sync.WaitGroup
	for _, svc := range hub.svcs {
		wg.Add(1)
		go func(svc *Service) {
			defer wg.Done()
			hub.nl.Printf("init service. name='%v'\n", svc.Name)

			svc.State = "init"
			if err := svc.controller.Init(); err != nil {
				svc.State = "init failed"
				hub.nl.Printf("init service failed. name='%v' err='%v'\n", svc.Name, err)
				return
			}

			svc.State = "running"
			err := svc.controller.Start()

			svc.State = "stopped"
			hub.nl.Printf("service stopped. name='%v' err='%v'\n", svc.Name, err)
		}(svc)
	}
	wg.Wait()
	hub.nl.Println("no service is running")
}

func (hub *Hub) RangeAllService(fn func(svc *Service)) {
	for _, svc := range hub.svcs {
		fn(svc)
	}
}

func (hub *Hub) GetAllServiceState() ([]ServiceState, error) {
	if len(hub.svcs) <= 0 {
		return nil, errors.New("NO SERVICES")
	}

	var reports []ServiceState
	for _, svc := range hub.svcs {
		reports = append(reports, svc.ServiceState)
	}
	return reports, nil
}
func (hub *Hub) GetAllServiceStateString() string {
	reports, err := hub.GetAllServiceState()
	if err != nil {
		return fmt.Sprintf("report service failed: %v\n", err)
	}

	buf := bytes.NewBufferString("report service:\n")
	utils.RenderAsciiTable(buf, reports,
		[]string{"index", "type", "name", "state", "listen", "target", "actives", "dones"},
		func(d interface{}, index int) []string {
			s := d.(ServiceState)
			return []string{
				fmt.Sprintf("%v", index),
				s.Type,
				s.Name,
				s.State,
				s.ListenURL,
				s.TargetURL,
				fmt.Sprintf("%v", s.Actives),
				fmt.Sprintf("%v", s.Dones),
			}
		},
	)
	return buf.String()
}

func (hub *Hub) ReportAllNetwork() ([]NetworkReport, error) {
	if len(hub.nets) <= 0 {
		return nil, errors.New("NO NETWORKS")
	}

	var reports []NetworkReport
	for _, nt := range hub.nets {
		reports = append(reports, nt.Report())
	}
	return reports, nil
}

func (hub *Hub) GetAllNetworkString() string {
	reports, err := hub.ReportAllNetwork()
	if err != nil {
		return fmt.Sprintf("report network failed: %v\n", err)
	}

	buf := bytes.NewBufferString("report network:\n")
	utils.RenderAsciiTable(buf, reports,
		[]string{"index", "name", "addr", "domain", "lsn", "dial"},
		func(d interface{}, index int) []string {
			s := d.(NetworkReport)
			return []string{
				fmt.Sprintf("%v", index),
				s.Name,
				s.Address,
				s.Domain,
				fmt.Sprintf("%v", s.Listens),
				fmt.Sprintf("%v", s.Dials),
			}
		},
	)
	return buf.String()
}

// AddNetwork 在hub中增加network
func (hub *Hub) AddNetwork(mnet Network) error {
	name := mnet.GetName()
	if name == "" {
		return errors.New("invalid network name=''")
	}
	hub.mut.Lock()
	defer hub.mut.Unlock()

	_, found := hub.nets[name]
	if found {
		return errors.New("network exists")
	}
	hub.nets[name] = mnet

	hub.nl.Printf("agent registered. name='%v'\n", name)
	return nil
}

// FindNetwork 获取网络
func (hub *Hub) FindNetwork(network string) (Network, error) {
	if network == "" {
		return nil, errors.New("invalid network name=''")
	}
	hub.mut.RLock()
	defer hub.mut.RUnlock()

	mnet, found := hub.nets[network]
	if !found {
		return nil, fmt.Errorf("network='%v' not found", network)
	}
	return mnet, nil
}

// Dial 创建连接
func (hub *Hub) Dial(network, addr string) (net.Conn, error) {
	mnet, err := hub.FindNetwork(network)
	if err != nil {
		return nil, err
	}
	return mnet.Dial(network, addr)
}

// URLDialer 对URL进行预处理，在调用时快速创建连接
func (hub *Hub) URLDialer(raw string) (QuickDialer, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	return func() (net.Conn, error) {
		return hub.dialu(u)
	}, nil
}

// DialURL 直接根据URL信息创建连接
func (hub *Hub) DialURL(raw string) (net.Conn, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return hub.dialu(u)
}

// dialu 根据url.URL对象信息创建连接
// - url.Scheme 对应 network
// - url.Host 对应 address
// - url.Query 对应其它控制参数，例如：加密、压缩等
func (hub *Hub) dialu(u *url.URL) (net.Conn, error) {
	c, err := hub.Dial(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	secret := u.Query().Get("secret")
	if secret == "" {
		return c, nil
	}
	c, err = cipherconn.New(c, secret)
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (hub *Hub) Listen(network, addr string) (net.Listener, error) {
	mnet, err := hub.FindNetwork(network)
	if err != nil {
		return nil, err
	}
	return mnet.Listen(network, addr)
}

func (hub *Hub) ListenURL(raw string) (net.Listener, error) {
	return ListenURL(hub, raw)
}

func ListenURL(network interface {
	Listen(network, addr string) (net.Listener, error)
}, raw string) (net.Listener, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	l, err := network.Listen(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}

	secret := u.Query().Get("secret")
	if secret == "" {
		return l, nil
	}

	return utils.NewSecretListener(l, secret), nil
}