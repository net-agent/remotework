package network

import (
	"net"
	"time"
)

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

type Network interface {
	Dial(network, addr string) (net.Conn, error)
	Listen(network, addr string) (net.Listener, error)
	Report() NodeReport
}
