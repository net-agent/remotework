package agent

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/remotework/utils"
	"github.com/olekukonko/tablewriter"
)

// tcp network wrap
type tcpnetwork struct {
	Type    string
	Listens int32
	Dials   int32
}

func (tcp *tcpnetwork) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}
func (tcp *tcpnetwork) Listen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}
func (tcp *tcpnetwork) Report() NodeReport {
	return NodeReport{
		Name: tcp.Type,
	}
}

type NetHub struct {
	nl   *utils.NamedLogger
	nets map[string]Network
	mut  sync.RWMutex

	svcs      []Service
	svcWaiter sync.WaitGroup
}

func NewNetHub() *NetHub {
	nets := make(map[string]Network)
	nets["tcp"] = &tcpnetwork{"tcp", 0, 0}
	nets["tcp4"] = &tcpnetwork{"tcp4", 0, 0}
	nets["tcp6"] = &tcpnetwork{"tcp6", 0, 0}

	return &NetHub{
		nl:   utils.NewNamedLogger("hub", false),
		nets: nets,
	}
}

func (hub *NetHub) TriggerNetworkUpdate(network string) {
	hub.nl.Printf("network='%v' updated.\n", network)
	for _, svc := range hub.svcs {
		if svc.Network() == network {
			go svc.Update()
		}
	}
}

func (hub *NetHub) AddServices(svcs ...Service) {
	for _, svc := range svcs {
		err := svc.Init()
		if err != nil {
			hub.nl.Printf("service init. name='%v' failed. err=%v\n", svc.Name(), err)
			continue
		}

		hub.svcs = append(hub.svcs, svc)
	}
}

func (hub *NetHub) StartServices() {
	for index, svc := range hub.svcs {
		hub.svcWaiter.Add(1)
		hub.nl.Printf("service running. name='%v' index=%v\n", svc.Name(), index)
		go func(svc Service) {
			defer hub.svcWaiter.Done()
			err := svc.Start()
			<-time.After(time.Millisecond * 100)
			hub.nl.Printf("service stopped. name='%v' err=%v\n", svc.Name(), err)
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
func (hub *NetHub) ServiceReportAscii(out *os.File) {
	reports, err := hub.ServiceReport()
	if err != nil {
		out.WriteString(fmt.Sprintf("ServiceReprotAscii failed: %v\n", err))
		return
	}

	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{"index", "name", "state", "listen", "target", "actives", "dones"})
	for index, info := range reports {
		table.Append([]string{
			fmt.Sprintf("%v", index),
			info.Name,
			info.State,
			info.Listen,
			info.Target,
			fmt.Sprintf("%v", info.Actives),
			fmt.Sprintf("%v", info.Dones),
		})
	}
	table.Render()
}

func (hub *NetHub) NetworkReport() ([]NodeReport, error) {
	if len(hub.nets) <= 0 {
		return nil, errors.New("NO NETWORKS")
	}

	var reports []NodeReport
	for _, nt := range hub.nets {
		reports = append(reports, nt.Report())
	}
	return reports, nil
}

func (hub *NetHub) NetworkReportAscii(out *os.File) {
	reports, err := hub.NetworkReport()
	if err != nil {
		out.WriteString(fmt.Sprintf("NetworkReportAscii failed: %v\n", err))
		return
	}

	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{"index", "name", "addr", "domain", "lsn", "dial"})
	for index, info := range reports {
		table.Append([]string{
			fmt.Sprintf("%v", index),
			info.Name,
			info.Address,
			info.Domain,
			fmt.Sprintf("%v", info.Listens),
			fmt.Sprintf("%v", info.Dials),
		})
	}
	table.Render()
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
