package agent

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/remotework/utils"
)

// NetworkRegistry 管理所有网络的注册、查找和连接工厂
type NetworkRegistry struct {
	nl   *utils.NamedLogger
	nets map[string]Network
	mut  sync.RWMutex
}

// newNetworkRegistryBare 创建空的 NetworkRegistry，不注册任何网络（供测试使用）
func newNetworkRegistryBare() *NetworkRegistry {
	return &NetworkRegistry{
		nl:   utils.NewNamedLogger("hub", false),
		nets: make(map[string]Network),
	}
}

func NewNetworkRegistry() *NetworkRegistry {
	nr := newNetworkRegistryBare()
	nr.Add(newTcpNetwork("tcp"))
	nr.Add(newTcpNetwork("tcp4"))
	nr.Add(newTcpNetwork("tcp6"))
	return nr
}

// Add 在注册表中增加network
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
	nr.nets[name] = mnet
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
	if network == "tcp" || network == "tcp4" || network == "tcp6" {
		return false
	}
	nr.mut.RLock()
	defer nr.mut.RUnlock()

	_, found := nr.nets[network]
	return found
}

func (nr *NetworkRegistry) StopAll() {
	nr.mut.RLock()
	nets := make(map[string]Network, len(nr.nets))
	for k, v := range nr.nets {
		nets[k] = v
	}
	nr.mut.RUnlock()

	for _, mnet := range nets {
		mnet.Stop()
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
		return nil, err
	}

	secret := u.Query().Get("secret")
	if nr.IsPrivateNetwork(u.Scheme) && secret == "" {
		l.Close()
		return nil, errors.New("to listen private protocols, please set the encryption password in the secret parameter")
	}

	if secret == "" {
		return l, nil
	}

	return utils.NewSecretListener(l, secret), nil
}

// URLDialer 对URL进行预处理，在调用时快速创建连接，实现 DialerFactory 接口
func (nr *NetworkRegistry) URLDialer(raw string) (QuickDialer, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
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
