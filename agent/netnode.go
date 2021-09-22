package agent

import (
	"errors"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/net-agent/flex/v2/node"
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
	connectFn ConnectFunc
	node      *node.Node
	nodeMut   sync.RWMutex

	Type      string
	Address   string
	Domain    string
	StartTime time.Time
	Listens   int32
	Accepts   int32
	Dials     int32
	Sends     int64
	Recvs     int64
}
type ConnectFunc func() (*node.Node, error)

func NewNetwork(info AgentInfo) *NetNode {
	n := &NetNode{
		connectFn: info.GetConnectFn(),

		Type:      info.Network,
		Domain:    info.Domain,
		StartTime: time.Now(),
	}

	if info.WsEnable {
		if info.Wss {
			n.Address = "wss://" + info.Address
		} else {
			n.Address = "ws://" + info.Address
		}
	} else {
		n.Address = "tcp://" + info.Address
	}

	return n
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

	if mnet.connectFn == nil {
		return nil, errors.New("need call SetConnectFunc first")
	}

	node, err := mnet.connectFn()
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

func (mnet *NetNode) SetConnectFunc(fn ConnectFunc) {
	mnet.connectFn = fn
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
			log.Printf("connect failed: %v\n", err)
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

		log.Printf("connect to server after %v\n", dur)
		<-time.After(dur)
	}
}
