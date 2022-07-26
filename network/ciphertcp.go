package network

import "net"

type cipherTcp struct {
	secret string
}

func (ct *cipherTcp) Dial(network, addr string) (net.Conn, error) {
	return nil, nil
}

func (ct *cipherTcp) Listen(network, addr string) (net.Listener, error) {
	return nil, nil
}

func (ct *cipherTcp) Report() NodeReport {
	return NodeReport{
		Type: "",
	}
}

func NewCipherTcp(secret string) Network {
	return &cipherTcp{}
}
