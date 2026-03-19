package vnet

import (
	"net"
	"time"
)

// tcpNetwork 包装标准 TCP 网络
type tcpNetwork struct {
	name string
}

func NewTcpNetwork(name string) *tcpNetwork {
	return &tcpNetwork{name: name}
}

func (tcp *tcpNetwork) GetName() string                             { return tcp.name }
func (tcp *tcpNetwork) Dial(network, addr string) (net.Conn, error) { return net.Dial(network, addr) }
func (tcp *tcpNetwork) Listen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}
func (tcp *tcpNetwork) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	return 0, ErrPingNotSupported
}
func (tcp *tcpNetwork) Meta() NetworkMeta {
	return NetworkMeta{
		Protocol: "-",
		Address:  "-",
		Domain:   "-",
		State:    "online",
		Kind:     "local",
	}
}
func (tcp *tcpNetwork) Stop() {}
