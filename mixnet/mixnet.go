package mixnet

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/net-agent/flex/node"
)

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

func (mnet *MixNet) Dial(network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4":
		return net.Dial(network, addr)
	case "flex":
		node, err := mnet.GetNode()
		if err != nil {
			return nil, err
		}
		return node.Dial(addr)
	default:
		return nil, fmt.Errorf("unknown network: %v", network)
	}
}

func (mnet *MixNet) Listen(network, addr string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4":
		return net.Listen(network, addr)
	case "flex":
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
		return node.Listen(uint16(port))
	default:
		return nil, fmt.Errorf("unknown network: %v", network)
	}
}

func (mnet *MixNet) GetNode() (*node.Node, error) {
	mnet.nodeMut.Lock()
	defer mnet.nodeMut.Unlock()

	if mnet.node == nil {
		mnet.nodeMut.Lock()
		if mnet.connectFn == nil {
			mnet.nodeMut.Unlock()
			return nil, errors.New("need call SetConnectFunc first")
		}

		node, err := mnet.connectFn()
		if err != nil {
			mnet.nodeMut.Unlock()
			return nil, err
		}
		mnet.node = node
		mnet.nodeMut.Unlock()
	}

	return mnet.node, nil
}

func (mnet *MixNet) SetConnectFunc(fn ConnectFunc) {
	mnet.connectFn = fn
}
