package agent

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/flex/v2/node"
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
}

func NewNetHub() *NetHub {
	nets := make(map[string]Network)
	tcp := &tcpnetwork{}
	nets["tcp"] = tcp
	nets["tcp4"] = tcp
	nets["tcp6"] = tcp

	return &NetHub{nets: nets}
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
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	l, err := hub.Listen(u.Scheme, u.Host)
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

type MixNet struct {
	connectFn ConnectFunc
	node      *node.Node
	nodeMut   sync.RWMutex
}
type ConnectFunc func() (*node.Node, error)

func NewNetwork(connectFn ConnectFunc) *MixNet {
	return &MixNet{
		connectFn: connectFn,
	}
}

func (mnet *MixNet) connect() (*node.Node, error) {
	if mnet.connectFn == nil {
		return nil, errors.New("should call SetConnectFunc first")
	}

	return mnet.connectFn()
}

func (mnet *MixNet) Dial(network, addr string) (net.Conn, error) {
	node, err := mnet.GetNode()
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("dial with nil node")
	}
	return node.Dial(addr)
}

func (mnet *MixNet) Listen(network, addr string) (net.Listener, error) {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	node, err := mnet.GetNode()
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("listen with nil node")
	}
	return node.Listen(uint16(port))
}

func (mnet *MixNet) GetNode() (*node.Node, error) {
	mnet.nodeMut.RLock()
	defer mnet.nodeMut.RUnlock()

	if mnet.node == nil {
		if mnet.connectFn == nil {
			return nil, errors.New("need call SetConnectFunc first")
		}
	}

	return mnet.node, nil
}

func (mnet *MixNet) SetConnectFunc(fn ConnectFunc) {
	mnet.connectFn = fn
}

func (mnet *MixNet) KeepAlive(evch chan struct{}) {
	dur := time.Second * 0
	for {
		if dur > time.Minute {
			dur = time.Minute
		}
		if dur > time.Millisecond {
			log.Printf("connect to server after %v\n\n", dur)
			<-time.After(dur)
		}

		var wg sync.WaitGroup

		mnet.nodeMut.Lock()
		node, err := mnet.connect()
		if err == nil && node != nil {
			mnet.node = node
			wg.Add(1)
			go func() {
				select {
				case evch <- struct{}{}:
				default:
				}
				node.Run()
				mnet.node = nil
				wg.Done()
			}()
		}
		mnet.nodeMut.Unlock()

		// 如果发生错误，打印错误，然后增加3秒停顿时间
		if err != nil {
			dur += time.Second * 3
			log.Printf("connect failed: %v\n", err)
			continue
		}

		// 等待node.Run返回，并根据执行时间判断停顿时长
		start := time.Now()
		wg.Wait()
		mnet.node = nil
		runDur := time.Since(start)
		if runDur > time.Second*27 {
			dur = time.Second * 3
		} else {
			// 确保至少30秒连接一次服务器。执行时间不足30秒的，需要等待
			dur = (time.Second * 30) - runDur
		}
	}

}
