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

type NetNode struct {
	connectFn ConnectFunc
	node      *node.Node
	nodeMut   sync.RWMutex
}
type ConnectFunc func() (*node.Node, error)

func NewNetwork(connectFn ConnectFunc) *NetNode {
	return &NetNode{
		connectFn: connectFn,
	}
}

func (mnet *NetNode) connect() (*node.Node, error) {
	if mnet.connectFn == nil {
		return nil, errors.New("should call SetConnectFunc first")
	}

	return mnet.connectFn()
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
	mnet.nodeMut.RLock()
	defer mnet.nodeMut.RUnlock()

	if mnet.node == nil {
		if mnet.connectFn == nil {
			return nil, errors.New("need call SetConnectFunc first")
		}
	}

	return mnet.node, nil
}

func (mnet *NetNode) SetConnectFunc(fn ConnectFunc) {
	mnet.connectFn = fn
}

func (mnet *NetNode) KeepAlive(evch chan struct{}) {
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
