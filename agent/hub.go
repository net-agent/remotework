package agent

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/remotework/utils"
)

type Hub struct {
	nl      *utils.NamedLogger
	nets    map[string]Network
	mut     sync.RWMutex
	running int32 // atomic: 0=stopped, 1=running

	svcs      []*Service
	svcNames  map[string]*Service
	svcMut    sync.RWMutex
	svcID     int32
	svcWaiter sync.WaitGroup
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
	var err error

	for _, info := range cfg.Agents {
		err = hub.AddNetwork(NewNetwork(hub, hub, info))
		if err != nil {
			hub.nl.Printf("network register failed. err='%v'\n", err)
		}
	}
	for _, info := range cfg.Portproxy {
		err = hub.AddService(NewPortproxyService(hub, info))
		if err != nil {
			hub.nl.Printf("service register failed. err='%v'\n", err)
		}
	}
	for _, info := range cfg.Socks5 {
		err = hub.AddService(NewSocks5Service(hub, info))
		if err != nil {
			hub.nl.Printf("service register failed. err='%v'\n", err)
		}
	}
	for _, info := range cfg.RDP {
		err = hub.AddService(NewRDPService(hub, info))
		if err != nil {
			hub.nl.Printf("service register failed. err='%v'\n", err)
		}
	}

	// load config summary
	networkNames := []string{}
	for name := range hub.nets {
		networkNames = append(networkNames, name)
	}
	hub.nl.Printf("registered networks: %v\n", strings.Join(networkNames, ", "))

	serviceNames := []string{}
	for name := range hub.svcNames {
		serviceNames = append(serviceNames, name)
	}
	hub.nl.Printf("registered services: %v\n", strings.Join(serviceNames, ", "))
}

func (hub *Hub) UpdateNetwork(network string) {
	count := 0
	hub.svcMut.RLock()
	svcs := make([]*Service, len(hub.svcs))
	copy(svcs, hub.svcs)
	hub.svcMut.RUnlock()

	for _, svc := range svcs {
		if svc.IsListenDepend(network) && svc.GetStatus() == StatusRunning {
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
		return errors.New("duplicate service name")
	}

	svc.SetStatus(StatusUninit)
	hub.svcs = append(hub.svcs, svc)
	hub.svcNames[svc.Name] = svc

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

func (hub *Hub) StartServices() error {
	if !atomic.CompareAndSwapInt32(&hub.running, 0, 1) {
		return errors.New("service is running")
	}
	defer atomic.StoreInt32(&hub.running, 0)

	hub.nl.Println("start services:")
	for _, svc := range hub.svcs {
		hub.StartService(svc)
	}

	hub.svcWaiter.Wait()
	hub.nl.Println("no service is running")
	return nil
}

func (hub *Hub) StartService(svc *Service) {
	st := svc.GetStatus()
	if st == StatusInit || st == StatusRunning {
		return
	}

	hub.svcWaiter.Add(1)
	go hub.manageServiceState(svc, &hub.svcWaiter)
}

func (hub *Hub) manageServiceState(svc *Service, waiter *sync.WaitGroup) {
	defer waiter.Done()
	hub.nl.Printf("init service. type='%v' name='%v' \n", svc.Type, svc.Name)

	svc.SetStatus(StatusInit)
	if err := svc.controller.Init(); err != nil {
		svc.SetStatus(StatusFailed)
		hub.nl.Printf("init service failed. name='%v' err='%v'\n", svc.Name, err)
		return
	}

	svc.SetStatus(StatusRunning)
	err := svc.controller.Start()
	svc.SetStatus(StatusStopped)

	hub.nl.Printf("service stopped. name='%v' err='%v'\n", svc.Name, err)
}

func (hub *Hub) StopServices() {
	if atomic.LoadInt32(&hub.running) == 0 {
		return
	}

	hub.svcMut.RLock()
	svcs := make([]*Service, len(hub.svcs))
	copy(svcs, hub.svcs)
	hub.svcMut.RUnlock()

	for _, svc := range svcs {
		if svc.GetStatus() == StatusRunning {
			svc.controller.Close()
		}
	}
}

func (hub *Hub) StopNetworks() {
	for _, mnet := range hub.nets {
		mnet.Stop()
	}
}

func (hub *Hub) IsRunning() bool { return atomic.LoadInt32(&hub.running) == 1 }

func (hub *Hub) RangeAllService(fn func(svc *Service)) {
	hub.svcMut.RLock()
	svcs := make([]*Service, len(hub.svcs))
	copy(svcs, hub.svcs)
	hub.svcMut.RUnlock()

	for _, svc := range svcs {
		fn(svc)
	}
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

func (hub *Hub) IsPrivateNetwork(network string) bool {
	if network == "" {
		return false
	}
	if network == "tcp" || network == "tcp4" || network == "tcp6" {
		return false
	}

	hub.mut.RLock()
	defer hub.mut.RUnlock()

	_, found := hub.nets[network]
	return found
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
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	l, err := hub.Listen(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}

	secret := u.Query().Get("secret")
	if hub.IsPrivateNetwork(u.Scheme) && secret == "" {
		l.Close()
		return nil, errors.New("to listen private protocols, please set the encryption password in the secret parameter")
	}

	if secret == "" {
		return l, nil
	}

	return utils.NewSecretListener(l, secret), nil
}

func (hub *Hub) PingDomain(network, domain string) (time.Duration, error) {
	mnet, err := hub.FindNetwork(network)
	if err != nil {
		return 0, err
	}
	return mnet.Ping(domain, time.Second*3)
}
