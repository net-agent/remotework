package agent

import (
	"net"
	"time"
)

// tcp network wrap
type tcpnetwork struct {
	networkinfo
	start time.Time
}

func newTcpNetwork(name string) *tcpnetwork {
	return &tcpnetwork{
		networkinfo: networkinfo{name: name},
		start:       time.Now(),
	}
}

func (tcp *tcpnetwork) Dial(network, addr string) (net.Conn, error) {
	tcp.addDialCount(1)
	return net.Dial(network, addr)
}
func (tcp *tcpnetwork) Listen(network, addr string) (net.Listener, error) {
	tcp.addListenCount(1)
	return net.Listen(network, addr)
}
func (tcp *tcpnetwork) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	return 0, ErrPingNotSupported
}
func (tcp *tcpnetwork) Report() NetworkReport {
	return NetworkReport{
		Name:    tcp.name,
		Protocol: "-",
		Address:  "-",
		Domain:   "-",
		Alive:   time.Since(tcp.start),
		Listens: tcp.getListenCount(),
		Accepts: 0,
		Dials:   tcp.getDialCount(),
		State:   "online",
	}
}
func (tcp *tcpnetwork) Stop() {}
