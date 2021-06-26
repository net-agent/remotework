package service

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/agent/netx"
)

type Portproxy struct {
	info agent.ServiceInfo

	closer io.Closer
	listen string
	target string
}

func NewPortproxy(info agent.ServiceInfo) *Portproxy {
	return &Portproxy{
		info: info,

		listen: info.Param["listen"],
		target: info.Param["target"],
	}
}

func (p *Portproxy) Info() string {
	if p.info.Enable {
		return green(fmt.Sprintf("%11v %24v %24v", p.info.Type, p.listen, p.target))
	}
	return yellow(fmt.Sprintf("%11v %24v", p.info.Type, "disabled"))
}

func (p *Portproxy) Run() error {
	if !p.info.Enable {
		return errors.New("service disabled")
	}
	l, err := netx.Listen(p.listen)
	if err != nil {
		return err
	}

	p.closer = l

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go p.serve(conn)
	}
}

func (p *Portproxy) Close() error {
	return p.closer.Close()
}

func (p *Portproxy) serve(c1 net.Conn) {
	c2, err := netx.Dial(p.target)
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
