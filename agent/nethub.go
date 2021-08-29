package agent

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
)

type QuickDialer func() (net.Conn, error)
type Network interface {
	Dial(network, addr string) (net.Conn, error)
	Listen(network, addr string) (net.Listener, error)
}

// tcp network wrap
type tcpnetwork struct {
}

func (tcp *tcpnetwork) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}
func (tcp *tcpnetwork) Listen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}

type NetHub struct {
	nets map[string]Network
	mut  sync.RWMutex

	svcs      []Service
	svcWaiter sync.WaitGroup
}

func NewNetHub() *NetHub {
	nets := make(map[string]Network)
	tcp := &tcpnetwork{}
	nets["tcp"] = tcp
	nets["tcp4"] = tcp
	nets["tcp6"] = tcp

	return &NetHub{nets: nets}
}

func (hub *NetHub) AddServices(svcs ...Service) {
	for _, svc := range svcs {
		err := svc.Init()
		if err != nil {
			log.Printf("[hub] service init. name='%v' failed. err=%v\n", svc.Name(), err)
			continue
		}

		hub.svcs = append(hub.svcs, svc)
	}
}

func (hub *NetHub) StartServices() {
	for _, svc := range hub.svcs {
		hub.svcWaiter.Add(1)
		log.Printf("[hub] service running. name='%v'\n", svc.Name())
		go func(svc Service) {
			defer hub.svcWaiter.Done()
			err := svc.Start()
			<-time.After(time.Millisecond * 100)
			log.Printf("[hub] service stopped. name='%v' err=%v\n", svc.Name(), err)
		}(svc)
	}
}

func (hub *NetHub) ServicesRange(fn func(svc Service)) {
	for _, svc := range hub.svcs {
		fn(svc)
	}
}

func (hub *NetHub) Wait() {
	hub.svcWaiter.Wait()
}

func (hub *NetHub) ServiceReport() ([]ReportInfo, error) {
	if len(hub.svcs) <= 0 {
		return nil, errors.New("NO SERVICES")
	}

	var reports []ReportInfo
	for _, svc := range hub.svcs {
		reports = append(reports, svc.Report())
	}
	return reports, nil
}

// AddNetwork 在hub中增加network
func (hub *NetHub) AddNetwork(network string, mnet Network) error {
	if network == "" {
		return errors.New("invalid network name=''")
	}
	hub.mut.Lock()
	defer hub.mut.Unlock()

	_, found := hub.nets[network]
	if found {
		return errors.New("network exists")
	}
	hub.nets[network] = mnet
	return nil
}

// GetNetwork 获取网络
func (hub *NetHub) GetNetwork(network string) (Network, error) {
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
func (hub *NetHub) Dial(network, addr string) (net.Conn, error) {
	mnet, err := hub.GetNetwork(network)
	if err != nil {
		return nil, err
	}
	return mnet.Dial(network, addr)
}

// URLDialer 对URL进行预处理，在调用时快速创建连接
func (hub *NetHub) URLDialer(raw string) (QuickDialer, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	return func() (net.Conn, error) {
		return hub.dialu(u)
	}, nil
}

// DialURL 直接根据URL信息创建连接
func (hub *NetHub) DialURL(raw string) (net.Conn, error) {
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
func (hub *NetHub) dialu(u *url.URL) (net.Conn, error) {
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

func (hub *NetHub) Listen(network, addr string) (net.Listener, error) {
	mnet, err := hub.GetNetwork(network)
	if err != nil {
		return nil, err
	}
	return mnet.Listen(network, addr)
}

func (hub *NetHub) ListenURL(raw string) (net.Listener, error) {
	return ListenURL(hub, raw)
}

func ListenURL(network Network, raw string) (net.Listener, error) {
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

	return newSecretListener(l, secret), nil
}

//
//
// Listener
//

type secretListener struct {
	net.Listener
	ch chan net.Conn
}

func newSecretListener(l net.Listener, secret string) net.Listener {
	ch := make(chan net.Conn, 128)
	go func() {
		var wg sync.WaitGroup
		for {
			conn, err := l.Accept()
			if err != nil {
				break
			}

			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				cc, err := cipherconn.New(c, secret)
				if err != nil {
					c.Close()
					return
				}
				select {
				case ch <- cc:
				case <-time.After(time.Second * 20):
				}
			}(conn)
		}
		wg.Wait() // wait all channel push done
		close(ch)
	}()

	sl := &secretListener{
		Listener: l,
		ch:       ch,
	}

	return sl
}

func (l *secretListener) Accept() (net.Conn, error) {
	c, ok := <-l.ch
	if !ok {
		return nil, errors.New("listener closed")
	}
	return c, nil
}
