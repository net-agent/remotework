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

type QuickDialer func() (net.Conn, error)
type Network interface {
	Dial(network, addr string) (net.Conn, error)
	Listen(network, addr string) (net.Listener, error)
	Report() NodeReport
}
type NodeReport struct {
	Type    string
	Address string
	Domain  string
	Alive   time.Duration
	Listens int32
	Accepts int32
	Dials   int32
	Sends   int64
	Recvs   int64
}
type NetNode struct {
	nl      *utils.NamedLogger
	node    *node.Node
	nodeMut sync.RWMutex

	Type      string
	Address   string
	URL       string
	Domain    string
	Password  string
	MacStr    string
	StartTime time.Time
	Listens   int32
	Accepts   int32
	Dials     int32
	Sends     int64
	Recvs     int64
}

func NewNetwork(info AgentInfo) *NetNode {
	n := &NetNode{
		nl: utils.NewNamedLogger(info.Network, true),

		Type:      info.Network,
		Domain:    info.Domain,
		Address:   info.Address,
		Password:  info.Password,
		MacStr:    utils.GetMacAddressStr(),
		StartTime: time.Now(),
	}

	if info.WsEnable {
		if info.Wss {
			n.URL = "wss://" + info.Address
		} else {
			n.URL = "ws://" + info.Address
		}
	} else {
		n.URL = "tcp://" + info.Address
	}

	return n
}

func (mnet *NetNode) connect() (*node.Node, error) {
	// step1: dial
	mnet.nl.Printf("connect to '%v'\n", mnet.URL)
	var pc packet.Conn
	var err error

	if strings.HasPrefix(mnet.URL, "ws") {
		var c *websocket.Conn
		c, _, err = websocket.DefaultDialer.Dial(mnet.URL, nil)
		if err == nil && c != nil {
			pc = packet.NewWithWs(c)
		}
	} else {
		var c net.Conn
		c, err = net.Dial("tcp4", mnet.Address)
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

	// step2: upgrade
	mnet.nl.Printf("upgrade as '%v'\n", mnet.Domain)
	node, err := switcher.UpgradeToNode(pc, mnet.Domain, mnet.MacStr, mnet.Password)
	if err != nil {
		pc.Close()
		return nil, err
	}

	return node, nil
}

func (mnet *NetNode) Report() NodeReport {
	return NodeReport{
		Type:    mnet.Type,
		Address: mnet.Address,
		Domain:  mnet.Domain,
		Alive:   time.Since(mnet.StartTime),
		Listens: mnet.Listens,
		Accepts: mnet.Accepts,
		Dials:   mnet.Dials,
		Sends:   mnet.Sends,
		Recvs:   mnet.Recvs,
	}
}

func (mnet *NetNode) Dial(network, addr string) (net.Conn, error) {
	node, err := mnet.GetNode()
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("dial with nil node")
	}
	return node.Dial(addr)
}

func (mnet *NetNode) Listen(network, addr string) (net.Listener, error) {
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

func (mnet *NetNode) GetNode() (*node.Node, error) {
	mnet.nodeMut.Lock()
	defer mnet.nodeMut.Unlock()

	if mnet.node != nil {
		return mnet.node, nil
	}

	node, err := mnet.connect()
	if err != nil {
		return nil, err
	}

	mnet.node = node
	return mnet.node, nil
}

func (mnet *NetNode) ResetNode() {
	mnet.nodeMut.Lock()
	defer mnet.nodeMut.Unlock()
	mnet.node = nil
}

func (mnet *NetNode) KeepAlive(evch chan struct{}) {
	dur := time.Second * 0
	minWaitDur := 3 * time.Second
	maxWaitDur := 1 * time.Minute
	minRunDur := 30 * time.Second // 至少执行30秒
	durStep := 3 * time.Second

	for {
		node, err := mnet.GetNode()
		if err != nil {
			// 如果发生错误，打印错误，然后增加3秒停顿时间
			dur += durStep
			mnet.nl.Printf("connect/upgrade failed: %v\n", err)
		} else {
			select {
			case evch <- struct{}{}:
			default:
			}

			// 等待node.Run返回，并根据执行时间判断停顿时长
			start := time.Now()
			node.Run()
			mnet.ResetNode()
			dur = minRunDur - time.Since(start)
		}

		if dur < minWaitDur {
			dur = minWaitDur
		} else if dur > maxWaitDur {
			dur = maxWaitDur
		}

		mnet.nl.Printf("connect to server after %v\n", dur)
		<-time.After(dur)
	}
}
