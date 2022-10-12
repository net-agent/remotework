package agent

import "net"

// tcp network wrap
type tcpnetwork struct {
	Type    string
	Listens int32
	Dials   int32
}

func (tcp *tcpnetwork) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}
func (tcp *tcpnetwork) Listen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}
func (tcp *tcpnetwork) Report() NetworkReport {
	return NetworkReport{
		Name: tcp.Type,
	}
}
