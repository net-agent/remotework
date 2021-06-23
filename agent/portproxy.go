package main

import (
	"io"
	"net"
)

type Portproxy struct {
	target string
}

func NewPortproxy(target string) *Portproxy {
	return &Portproxy{target}
}

func (p *Portproxy) Run(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go p.serve(conn)
	}
}

func (p *Portproxy) Close() error {
	return nil
}

func (p *Portproxy) serve(c1 net.Conn) {
	c2, err := dial(p.target)
	if err != nil {
		c1.Close()
		return
	}

	go func() {
		io.Copy(c2, c1)
		c1.Close()
		c2.Close()
	}()

	io.Copy(c1, c2)
	c1.Close()
	c2.Close()

}
