package agent

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v2/handshake"
	"github.com/net-agent/flex/v2/node"
	"github.com/net-agent/flex/v2/packet"
	"github.com/net-agent/remotework/utils"
)

var (
	ErrNodeClosed = errors.New("connect failed, node closed")
)

type networkImpl struct {
	networkinfo
	hub               *Hub
	nl                *utils.NamedLogger
	node              *node.Node
	onceInit          sync.Once
	nodeWaiter        chan *node.Node
	nodeWaiterTimeout time.Duration
	state             string
	lastErr           string
	closed            bool

	Name        string
	Protocol    string
	Address     string
	URL         string
	Domain      string
	Password    string
	MacStr      string
	ConnectTime time.Time
	Sends       int64
	Recvs       int64
}

func NewNetwork(hub *Hub, info AgentInfo) *networkImpl {
	n := &networkImpl{
		networkinfo:       networkinfo{name: info.Name},
		hub:               hub,
		nl:                utils.NewNamedLogger(info.Name, true),
		state:             "offline",
		lastErr:           "",
		nodeWaiter:        make(chan *node.Node),
		nodeWaiterTimeout: time.Second * 8,

		Name:        info.Name,
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
	mnet.closed = true
	if mnet.node != nil {
		mnet.node.Close()
	}
}

func (mnet *networkImpl) Report() NetworkReport {
	alive := time.Since(mnet.ConnectTime)
	if mnet.state != "online" {
		alive = 0
	}
	return NetworkReport{
		Name:     mnet.Name,
		Protocol: mnet.Protocol,
		Address:  mnet.Address,
		Domain:   mnet.Domain,
		Alive:    alive,
		Listens:  mnet.listenCount,
		Accepts:  0,
		Dials:    mnet.dialCount,
		Sends:    mnet.Sends,
		Recvs:    mnet.Recvs,
		State:    mnet.state,
		LastErr:  mnet.lastErr,
	}
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

// func (mnet *networkImpl) getNode() (*node.Node, error) {
// 	mnet.onceInit.Do(func() {
// 		ch := make(chan *node.Node, 1)
// 		mnet.nodeWaiter = ch
// 		go mnet.keepalive()
// 		<-mnet.nodeWaiter
// 		mnet.nodeWaiter = nil
// 		close(ch)
// 	})

// 	if mnet.node == nil {
// 		return nil, errors.New("node instance is null")
// 	}
// 	return mnet.node, nil
// }

func (mnet *networkImpl) getNodeInstance() (*node.Node, error) {
	// 第一步：初始化（只会执行一次）
	mnet.onceInit.Do(func() {
		go mnet.keepalive()
	})

	// 第二步：获取实例
	select {
	case node := <-mnet.nodeWaiter:
		if node == nil {
			return nil, errors.New("node is nil")
		}
		return node, nil
	case <-time.After(mnet.nodeWaiterTimeout):
		return nil, errors.New("wait node timeout")
	}
}

// keepalive 创建连接，并保持连接在线。出现异常时会不断尝试重连，直至连接成功为止
// 该方法在第一次尝试调用getNode时触发
// 每一次调用Dial和Listen时，都会调用getNode
func (mnet *networkImpl) keepalive() {
	cd := utils.NewCooldown(3*time.Second, 1*time.Minute)

	for {
		mnet.state = "connecting"
		node, err := mnet.connect()
		cd.Tick() // 开始冷却计时

		if err == ErrNodeClosed {
			mnet.state = "closed"
			mnet.nl.Println("network closed")
			return
		}

		if err != nil {
			mnet.state = "offline"
			mnet.lastErr = err.Error()

			mnet.nl.Printf("connect '%v' failed: %v, retry after %v\n", mnet.name, err, cd.WaitDuration())

			<-cd.Wait()
			cd.Increase(3 * time.Second) // 连接失败后等待时间增加3秒
		} else {
			mnet.ConnectTime = time.Now()
			mnet.state = "online"
			mnet.lastErr = ""

			mnet.node = node
			closeCtx, cancel := context.WithCancel(context.Background())
			go func() {
				for {
					select {
					case mnet.nodeWaiter <- node:
					case <-closeCtx.Done():
						return
					}
				}
			}()

			// mnet.node更新后，需要通知hub，更新相应的service依赖
			mnet.hub.UpdateNetwork(mnet.Name)

			// 连接成功后设置等待时间为30秒，至少30秒后才会开始重连
			cd.Set(30 * time.Second)
			node.Run() // 正常情况下这里会阻塞住

			cancel()
			mnet.state = "offline"
			mnet.node = nil

			mnet.nl.Printf("reconnect '%v' after %v\n", mnet.name, cd.WaitDuration())
			<-cd.Wait()
			cd.Reset() // 清零等待的叠加时间
		}

	}
}

// connect 连接中转服务器，创建会话。每次断线后需要重新调用
func (mnet *networkImpl) connect() (*node.Node, error) {
	if mnet.closed {
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
		c, err = mnet.hub.Dial(mnet.Protocol, mnet.Address)
		// c, err = net.Dial("tcp4", mnet.Address)
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

	node := node.New(pc)
	node.SetDomain(mnet.Domain)
	node.SetIP(ip)
	return node, nil
}
