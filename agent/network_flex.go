package agent

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v2/node"
	"github.com/net-agent/flex/v2/packet"
	"github.com/net-agent/flex/v2/switcher"
	"github.com/net-agent/remotework/utils"
)

type networkImpl struct {
	networkinfo
	hub        *Hub
	nl         *utils.NamedLogger
	node       *node.Node
	onceInit   sync.Once
	nodeWaiter chan *node.Node
	state      string
	lastErr    string

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
		networkinfo: networkinfo{name: info.Name},
		hub:         hub,
		nl:          utils.NewNamedLogger(info.Name, true),
		state:       "offline",
		lastErr:     "",

		Name:        info.Name,
		Protocol:    info.Protocol,
		Domain:      info.Domain,
		Address:     info.Address,
		Password:    info.Password,
		MacStr:      utils.GetMacAddressStr(),
		ConnectTime: time.Now(),
	}

	if info.WsEnable {
		info.Protocol = "ws"
		if info.Wss {
			info.Protocol = "wss"
		}
	}
	n.URL = fmt.Sprintf("%v://%v%v", info.Protocol, info.Address, info.WsPath)

	return n
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
	node, err := mnet.getNode()
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
	return mnet.node.PingDomain(domain, timeout)
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

	node, err := mnet.getNode()
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("listen with nil node")
	}
	mnet.addListenCount(1)
	return node.Listen(uint16(port))
}

func (mnet *networkImpl) getNode() (*node.Node, error) {
	mnet.onceInit.Do(func() {
		ch := make(chan *node.Node, 1)
		mnet.nodeWaiter = ch
		go mnet.keepalive()
		<-mnet.nodeWaiter
		mnet.nodeWaiter = nil
		close(ch)
	})

	if mnet.node == nil {
		return nil, errors.New("node instance is null")
	}
	return mnet.node, nil
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

		if err != nil {
			mnet.state = "offline"
			mnet.lastErr = err.Error()

			mnet.nl.Printf("connect failed: %v, retry after %v\n", err, cd.WaitDuration())

			cd.Wait()
			cd.Increase(3 * time.Second) // 连接失败后等待时间增加3秒
		} else {
			mnet.ConnectTime = time.Now()
			mnet.state = "online"
			mnet.lastErr = ""

			mnet.node = node
			if mnet.nodeWaiter != nil {
				mnet.nodeWaiter <- node
			}

			// mnet.node更新后，需要通知hub，更新相应的service依赖
			mnet.hub.UpdateNetwork(mnet.Name)

			// 连接成功后设置等待时间为30秒，至少30秒后才会开始重连
			cd.Set(30 * time.Second)
			node.Run() // 正常情况下这里会阻塞住
			mnet.state = "offline"
			mnet.node = nil

			mnet.nl.Printf("retry after %v\n", cd.WaitDuration())
			cd.Wait()
			cd.Reset() // 清零等待的叠加时间
		}

	}
}

// connect 连接中转服务器，创建会话。每次断线后需要重新调用
func (mnet *networkImpl) connect() (*node.Node, error) {
	// step1: 尝试通过tcp或ws连接中转服务
	var pc packet.Conn
	var err error

	if strings.HasPrefix(mnet.URL, "ws") {
		mnet.nl.Printf("connect to '%v'\n", mnet.URL)
		var c *websocket.Conn
		c, _, err = websocket.DefaultDialer.Dial(mnet.URL, nil)
		if err == nil && c != nil {
			pc = packet.NewWithWs(c)
		}
	} else {
		mnet.nl.Printf("connect to '%v'\n", mnet.Address)
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
	mnet.nl.Printf("upgrade as '%v'\n", mnet.Domain)
	node, err := switcher.UpgradeToNode(pc, mnet.Domain, mnet.MacStr, mnet.Password)
	if err != nil {
		pc.Close()
		return nil, err
	}

	return node, nil
}
