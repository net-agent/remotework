package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/net-agent/remotework/agent"
)

type Portproxy struct {
	mnet *agent.MixNet
	info agent.ServiceInfo

	closer       io.Closer
	listen       string
	target       string
	targetDialer agent.Dialer
}

func NewPortproxy(mnet *agent.MixNet, info agent.ServiceInfo) *Portproxy {
	target := info.Param["target"]
	return &Portproxy{
		mnet: mnet,
		info: info,

		listen:       info.Param["listen"],
		target:       target,
		targetDialer: mnet.URLDialer(target),
	}
}

func (p *Portproxy) Info() string {
	if p.info.Enable {
		return agent.Green(fmt.Sprintf("%11v %24v %24v", p.info.Type, p.listen, p.target))
	}
	return agent.Yellow(fmt.Sprintf("%11v %24v", p.info.Type, "disabled"))
}

func (p *Portproxy) Run() error {
	if !p.info.Enable {
		return errors.New("service disabled")
	}

	l, err := p.mnet.ListenURL(p.listen)
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
	c2, err := p.targetDialer()
	if err != nil {
		log.Printf("[portproxy] dial %v failed: %v\n", p.target, err)
		c1.Close()
		return
	}
	log.Printf("[portproxy] serve %v -> %v -> %v\n", c1.RemoteAddr(), p.listen, p.target)

	go func() {
		io.Copy(c2, c1)
		c1.Close()
		c2.Close()
	}()

	io.Copy(c1, c2)
	c1.Close()
	c2.Close()
}
