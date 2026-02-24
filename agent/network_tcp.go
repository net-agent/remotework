package agent

import (
	"net"
	"time"
)

// tcp network wrap
type tcpnetwork struct {
	name string
}

func newTcpNetwork(name string) *tcpnetwork {
	return &tcpnetwork{name: name}
}

func (tcp *tcpnetwork) GetName() string { return tcp.name }
func (tcp *tcpnetwork) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}
func (tcp *tcpnetwork) Listen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}
func (tcp *tcpnetwork) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	return 0, ErrPingNotSupported
}
func (tcp *tcpnetwork) Meta() NetworkMeta {
	return NetworkMeta{
		Protocol: "-",
		Address:  "-",
		Domain:   "-",
		State:    "online",
	}
}
func (tcp *tcpnetwork) Stop() {}
