package network

import (
	"crypto/tls"
	"net"
)

// tcp network wrap
type tlsnet struct {
	tlsListenConfig *tls.Config
	tlsDialConfig   *tls.Config
	Type            string
	Listens         int32
	Dials           int32
}

func (t *tlsnet) Dial(network, addr string) (net.Conn, error) {
	return tls.Dial(network, addr, t.tlsDialConfig)
}

func (t *tlsnet) Listen(network, addr string) (net.Listener, error) {
	return tls.Listen(network, addr, t.tlsListenConfig)
}

func (t *tlsnet) Report() NodeReport {
	return NodeReport{
		Type: t.Type,
	}
}

func NewTls(certPath, keyPath string) (Network, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	return &tlsnet{
		tlsListenConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		tlsDialConfig: &tls.Config{},
		Type:          "tls",
		Listens:       0,
		Dials:         0,
	}, nil
}
