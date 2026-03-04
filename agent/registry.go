package agent

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/agent/vnet"
	"github.com/net-agent/remotework/utils"
)

// NetworkRegistry 管理所有网络的注册、查找和连接工厂
type NetworkRegistry struct {
	log  *slog.Logger
	nets map[string]*vnet.ReportingNetwork
	mut  sync.RWMutex
}

// NewNetworkRegistryBare 创建空的 NetworkRegistry，不注册任何网络（供测试使用）
func NewNetworkRegistryBare(log *slog.Logger) *NetworkRegistry {
	if log == nil {
		log = utils.NewModuleLogger("hub.net")
	}
	return &NetworkRegistry{
		log:  log,
		nets: make(map[string]*vnet.ReportingNetwork),
	}
}

func NewNetworkRegistry(log *slog.Logger) *NetworkRegistry {
	nr := NewNetworkRegistryBare(log)
	nr.Add(vnet.NewTcpNetwork("tcp"))
	nr.Add(vnet.NewTcpNetwork("tcp4"))
	nr.Add(vnet.NewTcpNetwork("tcp6"))
	return nr
}

// Add 在注册表中增加network，自动包装为 reportingNetwork
func (nr *NetworkRegistry) Add(mnet Network) error {
	name := mnet.GetName()
	if name == "" {
		return errors.New("invalid network name=''")
	}
	nr.mut.Lock()
	defer nr.mut.Unlock()

	if _, found := nr.nets[name]; found {
		return errors.New("network exists")
	}
	nr.nets[name] = vnet.NewReportingNetwork(mnet)
	return nil
}

// Find 获取网络
func (nr *NetworkRegistry) Find(network string) (Network, error) {
	if network == "" {
		return nil, errors.New("invalid network name=''")
	}
	nr.mut.RLock()
	defer nr.mut.RUnlock()

	mnet, found := nr.nets[network]
	if !found {
		return nil, fmt.Errorf("network='%v' not found", network)
	}
	return mnet, nil
}

func (nr *NetworkRegistry) IsPrivateNetwork(network string) bool {
	if network == "" {
		return false
	}
	if nr.isBuiltinNetwork(network) {
		return false
	}
	nr.mut.RLock()
	defer nr.mut.RUnlock()

	_, found := nr.nets[network]
	return found
}

// isBuiltinNetwork 判断是否为内置网络（tcp/tcp4/tcp6）
func (nr *NetworkRegistry) isBuiltinNetwork(network string) bool {
	return network == "tcp" || network == "tcp4" || network == "tcp6"
}

func (nr *NetworkRegistry) StopAll() {
	nr.mut.RLock()
	nets := make([]*vnet.ReportingNetwork, 0, len(nr.nets))
	for _, v := range nr.nets {
		nets = append(nets, v)
	}
	nr.mut.RUnlock()

	for _, rn := range nets {
		rn.Stop()
	}
}

// Dial 创建连接
func (nr *NetworkRegistry) Dial(network, addr string) (net.Conn, error) {
	mnet, err := nr.Find(network)
	if err != nil {
		return nil, err
	}
	return mnet.Dial(network, addr)
}

// Listen 创建监听
func (nr *NetworkRegistry) Listen(network, addr string) (net.Listener, error) {
	mnet, err := nr.Find(network)
	if err != nil {
		return nil, err
	}
	return mnet.Listen(network, addr)
}

// ListenURL 根据URL创建监听，实现 ListenerFactory 接口
func (nr *NetworkRegistry) ListenURL(raw string) (net.Listener, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	l, err := nr.Listen(u.Scheme, u.Host)
	if err != nil {
		if !nr.isBuiltinNetwork(u.Scheme) {
			return nil, &ErrDependencyNotReady{Network: u.Scheme}
		}
		return nil, err
	}

	// 传输层加密：vtcp 端点通过 authcode 参数启用 cipherconn
	authcode := u.Query().Get("authcode")
	if authcode == "" {
		return l, nil
	}

	return utils.NewSecretListener(l, authcode), nil
}

// URLDialer 对URL进行预处理，在调用时快速创建连接，实现 DialerFactory 接口
func (nr *NetworkRegistry) URLDialer(raw string) (QuickDialer, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	// 预��查网络是否存在，对非内置网络返回依赖未就绪错误
	if _, err := nr.Find(u.Scheme); err != nil && !nr.isBuiltinNetwork(u.Scheme) {
		return nil, &ErrDependencyNotReady{Network: u.Scheme}
	}

	return func() (net.Conn, error) {
		return nr.dialu(u)
	}, nil
}

// DialURL 直接根据URL信息创建连接
func (nr *NetworkRegistry) DialURL(raw string) (net.Conn, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return nr.dialu(u)
}

// dialu 根据url.URL对象信息创建连接
func (nr *NetworkRegistry) dialu(u *url.URL) (net.Conn, error) {
	c, err := nr.Dial(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}
	authcode := u.Query().Get("authcode")
	if authcode == "" {
		return c, nil
	}
	c, err = cipherconn.New(c, authcode)
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (nr *NetworkRegistry) PingDomain(network, domain string) (time.Duration, error) {
	mnet, err := nr.Find(network)
	if err != nil {
		return 0, err
	}
	return mnet.Ping(domain, time.Second*3)
}

// Names 返回所有已注册的网络名称
func (nr *NetworkRegistry) Names() []string {
	nr.mut.RLock()
	defer nr.mut.RUnlock()

	names := make([]string, 0, len(nr.nets))
	for name := range nr.nets {
		names = append(names, name)
	}
	return names
}

//
// NetworkRegistry 状态报告方法
//

func (nr *NetworkRegistry) GetAllState() ([]NetworkReport, error) {
	nr.mut.RLock()
	defer nr.mut.RUnlock()

	if len(nr.nets) <= 0 {
		return nil, errors.New("NO NETWORKS")
	}

	var reports []NetworkReport
	for _, nt := range nr.nets {
		reports = append(reports, nt.Report())
	}
	return reports, nil
}

func (nr *NetworkRegistry) GetAllStateString() string {
	reports, err := nr.GetAllState()
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

// streamStateProvider 用于获取数据流状态的可选接口
type streamStateProvider interface {
	GetStreamStates() (actives, closeds []*stream.State)
}

func getDataStreamStateByNetwork(mnet Network) (actives, closeds []*stream.State) {
	// 穿透装饰器访问内部 Network
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

type DataStreamState struct {
	Network string
	Actives []*stream.State
	Closeds []*stream.State
}

func (nr *NetworkRegistry) GetAllDataStreamStateString() string {
	buf := bytes.NewBufferString("report actived stream:\n")

	nr.mut.RLock()
	nets := make(map[string]*vnet.ReportingNetwork, len(nr.nets))
	for k, v := range nr.nets {
		nets[k] = v
	}
	nr.mut.RUnlock()

	for networkName, rn := range nets {
		states, _ := getDataStreamStateByNetwork(rn)
		if len(states) > 0 {
			utils.RenderAsciiTable(buf, states,
				[]string{"index", "network", "local", "remote", "readed", "wrote", "alive"},
				func(d interface{}, index int) []string {
					st := d.(*stream.State)
					alived := time.Since(st.Created)
					if st.IsClosed {
						alived = st.Closed.Sub(st.Created)
					}
					return []string{
						fmt.Sprint(index),
						networkName,
						fmt.Sprintf("%v(%v)", st.LocalDomain, st.LocalAddr.String()),
						fmt.Sprintf("%v(%v)", st.RemoteDomain, st.RemoteAddr.String()),
						fmt.Sprint(0), // todo: st.ConnReadSize
						fmt.Sprint(0), // todo: st.ConnWriteSize
						fmt.Sprint(alived),
					}
				},
			)
		}
	}
	return buf.String()
}

func (nr *NetworkRegistry) GetDataStreamState(limits int, networks ...string) []*DataStreamState {
	resp := []*DataStreamState{}
	for _, network := range networks {
		mnet, err := nr.Find(network)
		if err != nil {
			resp = append(resp, nil)
			continue
		}

		actives, closeds := getDataStreamStateByNetwork(mnet)
		size := len(closeds)
		if size > limits {
			closeds = closeds[size-limits : size]
		}
		resp = append(resp, &DataStreamState{network, actives, closeds})
	}

	return resp
}
