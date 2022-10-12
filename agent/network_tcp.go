package agent

import "net"

// tcp network wrap
type tcpnetwork struct {
	networkinfo
	Type    string
	Listens int32
	Dials   int32
}

func newTcpNetwork(name string) *tcpnetwork {
	return &tcpnetwork{
		networkinfo: networkinfo{name: name},
		Type:        name,
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
func (tcp *tcpnetwork) Report() NetworkReport {
	return NetworkReport{
		Name: tcp.Type,
	}
}
