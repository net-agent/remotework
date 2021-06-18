package main

import (
	"io"
	"net"

	"github.com/net-agent/flex"
)

type Portproxy struct {
	host   *flex.Host
	target string
}

func NewPortproxy(host *flex.Host, target string) *Portproxy {
	return &Portproxy{host, target}
}

func (p *Portproxy) Run(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		p.serve(conn)
	}
}

func (p *Portproxy) Close() error {
	return nil
}

func (p *Portproxy) serve(c1 net.Conn) {
	defer c1.Close()

	c2, err := dial(p.host, p.target)
	if err != nil {
		return
	}
	defer c2.Close()

	go io.Copy(c2, c1)
	io.Copy(c1, c2)
}
