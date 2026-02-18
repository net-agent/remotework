package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v2/handshake"
	"github.com/net-agent/flex/v2/node"
	"github.com/net-agent/flex/v2/packet"
	"github.com/net-agent/flex/v2/stream"
	"github.com/net-agent/remotework/utils"
)

var (
	ErrNodeClosed = errors.New("connect failed, node closed")
)

type networkImpl struct {
	networkinfo
	rawDialer  RawDialer
	notifier   NetworkUpdateNotifier
	nl                *utils.NamedLogger
	onceInit          sync.Once
	nodeWaiter        chan *node.Node
	nodeWaiterTimeout time.Duration

	mu      sync.RWMutex // 保护 node, state, lastErr, closed, ConnectTime
	node    *node.Node
	state   string
	lastErr string
	closed  bool

	Protocol    string
	Address     string
	URL         string
	Domain      string
	Password    string
	MacStr      string
	ConnectTime time.Time
}

func NewNetwork(rd RawDialer, notifier NetworkUpdateNotifier, info AgentInfo) *networkImpl {
	n := &networkImpl{
		networkinfo:       networkinfo{name: info.Name},
		rawDialer:         rd,
		notifier:          notifier,
		nl:                utils.NewNamedLogger(info.Name, true),
		state:             "offline",
		lastErr:           "",
		nodeWaiter:        make(chan *node.Node),
		nodeWaiterTimeout: time.Second * 8,

		Protocol:    info.Protocol,
		Domain:      info.Domain,
		Address:     info.Address,
		Password:    info.Password,
		MacStr:      utils.GetMacAddressStr(),
		ConnectTime: time.Now(),
	}

	n.URL = fmt.Sprintf("%v://%v%v", info.Protocol, info.Address, info.WsPath)

	return n
}

func (mnet *networkImpl) Stop() {
	mnet.mu.Lock()
	mnet.closed = true
	n := mnet.node
	mnet.mu.Unlock()
	if n != nil {
		n.Close()
	}
}

func (mnet *networkImpl) isClosed() bool {
	mnet.mu.RLock()
	defer mnet.mu.RUnlock()
	return mnet.closed
}

func (mnet *networkImpl) setState(state, lastErr string) {
	mnet.mu.Lock()
	defer mnet.mu.Unlock()
	mnet.state = state
	if lastErr != "" {
		mnet.lastErr = lastErr
	}
}

func (mnet *networkImpl) setOnline(n *node.Node) {
	mnet.mu.Lock()
	defer mnet.mu.Unlock()
	mnet.node = n
	mnet.state = "online"
	mnet.lastErr = ""
	mnet.ConnectTime = time.Now()
}

func (mnet *networkImpl) clearNode() {
	mnet.mu.Lock()
	defer mnet.mu.Unlock()
	mnet.node = nil
	mnet.state = "offline"
}

func (mnet *networkImpl) Report() NetworkReport {
	mnet.mu.RLock()
	state := mnet.state
	lastErr := mnet.lastErr
	connectTime := mnet.ConnectTime
	mnet.mu.RUnlock()

	alive := time.Since(connectTime)
	if state != "online" {
		alive = 0
	}
	return NetworkReport{
		Name:     mnet.name,
		Protocol: mnet.Protocol,
		Address:  mnet.Address,
		Domain:   mnet.Domain,
		Alive:    alive,
		Listens:  mnet.getListenCount(),
		Accepts:  0,
		Dials:    mnet.getDialCount(),
		State:    state,
		LastErr:  lastErr,
	}
}

func (mnet *networkImpl) getNode() *node.Node {
	mnet.mu.RLock()
	defer mnet.mu.RUnlock()
	return mnet.node
}

// GetStreamStates 实现 streamStateProvider 接口
func (mnet *networkImpl) GetStreamStates() (actives, closeds []*stream.State) {
	n := mnet.getNode()
	if n == nil {
		return nil, nil
	}
	actives = n.GetStreamStateList()
	closeds = n.GetClosedStreamStateList(0)
	return actives, closeds
}

func (mnet *networkImpl) Dial(network, addr string) (net.Conn, error) {
	node, err := mnet.getNodeInstance()
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("dial with nil node")
	}
	mnet.addDialCount(1)
	return node.Dial(addr)
}

func (mnet *networkImpl) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	node, err := mnet.getNodeInstance()
	if err != nil {
		return 0, err
	}
	return node.PingDomain(domain, timeout)
}

func (mnet *networkImpl) Listen(network, addr string) (net.Listener, error) {
	hostname, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if hostname != "" && hostname != "0" && hostname != "local" && hostname != "localhost" {
		return nil, errors.New("invalid listen hostname")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	node, err := mnet.getNodeInstance()
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("listen with nil node")
	}
	mnet.addListenCount(1)
	return node.Listen(uint16(port))
}

func (mnet *networkImpl) getNodeInstance() (*node.Node, error) {
	// 第一步：初始化（只会执行一次）
	mnet.onceInit.Do(func() {
		go mnet.keepalive()
	})

	// 第二步：获取实例
	timer := time.NewTimer(mnet.nodeWaiterTimeout)
	defer timer.Stop()

	select {
	case node := <-mnet.nodeWaiter:
		if node == nil {
			return nil, errors.New("node is nil")
		}
		return node, nil
	case <-timer.C:
		return nil, errors.New("wait node timeout")
	}
}

// keepalive 创建连接，并保持连接在线。出现异常时会不断尝试重连，直至连接成功为止
// 该方法在第一次尝试调用getNodeInstance时触发
// 每一次调用Dial和Listen时，都会调用getNodeInstance
func (mnet *networkImpl) keepalive() {
	cd := utils.NewCooldown(3*time.Second, 1*time.Minute)

	for {
		mnet.setState("connecting", "")
		n, err := mnet.connect()
		cd.Tick() // 开始冷却计时

		if err == ErrNodeClosed {
			mnet.setState("closed", "")
			mnet.nl.Println("network closed")
			return
		}

		if err != nil {
			mnet.setState("offline", err.Error())

			mnet.nl.Printf("connect '%v' failed: %v, retry after %v\n", mnet.name, err, cd.WaitDuration())

			<-cd.Wait()
			cd.Increase(3 * time.Second) // 连接失败后等待时间增加3秒
		} else {
			mnet.setOnline(n)

			closeCtx, cancel := context.WithCancel(context.Background())
			go func() {
				for {
					select {
					case mnet.nodeWaiter <- n:
					case <-closeCtx.Done():
						return
					}
				}
			}()

			// mnet.node更新后，需要通知依赖方更新相应的service
			mnet.notifier.UpdateNetwork(mnet.name)

			// 连接成功后设置等待时间为30秒，至少30秒后才会开始重连
			cd.Set(30 * time.Second)
			n.Run() // 正常情况下这里会阻塞住

			cancel()
			mnet.clearNode()

			mnet.nl.Printf("reconnect '%v' after %v\n", mnet.name, cd.WaitDuration())
			<-cd.Wait()
			cd.Reset() // 清零等待的叠加时间
		}
	}
}

// connect 连接中转服务器，创建会话。每次断线后需要重新调用
func (mnet *networkImpl) connect() (*node.Node, error) {
	if mnet.isClosed() {
		return nil, ErrNodeClosed
	}
	// step1: 尝试通过tcp或ws连接中转服务
	var pc packet.Conn
	var err error

	if strings.HasPrefix(mnet.URL, "ws") {
		mnet.nl.Printf("dial to '%v'\n", mnet.URL)
		var c *websocket.Conn
		c, _, err = websocket.DefaultDialer.Dial(mnet.URL, nil)
		if err == nil && c != nil {
			pc = packet.NewWithWs(c)
		}
	} else {
		mnet.nl.Printf("dial to '%v'\n", mnet.Address)
		var c net.Conn
		c, err = mnet.rawDialer.Dial(mnet.Protocol, mnet.Address)
		if err == nil && c != nil {
			pc = packet.NewWithConn(c)
		}
	}

	if err != nil {
		return nil, err
	}

	if pc == nil {
		return nil, fmt.Errorf("connect failed with no error")
	}

	// step2: 通过upgrade对连接进行认证升级
	mnet.nl.Printf("upgrade as '%v://%v'\n", mnet.name, mnet.Domain)
	ip, err := handshake.UpgradeRequest(pc, mnet.Domain, mnet.MacStr, mnet.Password)
	if err != nil {
		pc.Close()
		return nil, err
	}

	n := node.New(pc)
	n.SetDomain(mnet.Domain)
	n.SetIP(ip)
	return n, nil
}
