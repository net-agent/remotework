package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/flex/stream"
	"github.com/net-agent/remotework/agent"
)

type Portproxy struct {
	mnet *agent.MixNet
	info agent.ServiceInfo

	closer       io.Closer
	listen       string
	target       string
	targetDialer agent.Dialer
	encode       string
	decode       string
}

func NewPortproxy(mnet *agent.MixNet, info agent.ServiceInfo) *Portproxy {
	target := info.Param["target"]
	return &Portproxy{
		mnet: mnet,
		info: info,

		listen:       info.Param["listen"],
		target:       target,
		targetDialer: mnet.URLDialer(target),

		encode: info.Param["encode"],
		decode: info.Param["decode"],
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
	var dialer string
	if s, ok := c1.(*stream.Conn); ok {
		dialer = "flex://" + s.Dialer()
	} else {
		dialer = "tcp://" + c1.RemoteAddr().String()
	}

	var err error
	if p.encode != "" {
		c1, err = cipherconn.New(c1, p.encode)
		if err != nil {
			log.Printf("[%v] encode failed. incorrect decode param in dialer='%v'\n", p.info.Type, dialer)
			c1.Close()
			return
		}
	}

	c2, err := p.targetDialer()
	if err != nil {
		log.Printf("[%v] dial target='%v' failed. %v\n", p.info.Type, p.target, err)
		c1.Close()
		return
	}
	if p.decode != "" {
		c2, err = cipherconn.New(c2, p.decode)
		if err != nil {
			log.Printf("[%v] decode failed. incorrect decode param in target='%v'\n", p.info.Type, p.target)
			c2.Close()
			c1.Close()
			return
		}
	}

	log.Printf("[%v] connect, dialer='%v' target='%v'\n", p.info.Type, dialer, p.target)

	go func() {
		io.Copy(c2, c1)
		c1.Close()
		c2.Close()
	}()

	io.Copy(c1, c2)
	c1.Close()
	c2.Close()
}
