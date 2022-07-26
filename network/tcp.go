package network

import "net"

// tcp network wrap
type tcp struct {
	Type    string
	Listens int32
	Dials   int32
}

func (tcp *tcp) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}
func (tcp *tcp) Listen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}
func (tcp *tcp) Report() NodeReport {
	return NodeReport{
		Type: tcp.Type,
	}
}

func NewTcp() Network  { return &tcp{"tcp", 0, 0} }
func NewTcp4() Network { return &tcp{"tcp4", 0, 0} }
func NewTcp6() Network { return &tcp{"tcp6", 0, 0} }
