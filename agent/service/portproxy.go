package service

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/net-agent/remotework/agent"
)

type Portproxy struct {
	mnet *agent.MixNet
	info agent.ServiceInfo

	closer        io.Closer
	listen        string
	listenNetwork string
	listenAddr    string
	target        string
	targetNetwork string
	targetAddr    string
}

func NewPortproxy(mnet *agent.MixNet, info agent.ServiceInfo) *Portproxy {
	listenNetwork, listenAddr, err := ParseAddr(info.Param["listen"])
	if err != nil {
		listenNetwork = "parseAddr failed: " + err.Error()
	}
	targetNetwork, targetAddr, err := ParseAddr(info.Param["target"])
	if err != nil {
		targetNetwork = "parseAddr failed: " + err.Error()
	}
	return &Portproxy{
		mnet: mnet,
		info: info,

		listen:        info.Param["listen"],
		listenNetwork: listenNetwork,
		listenAddr:    listenAddr,
		target:        info.Param["target"],
		targetNetwork: targetNetwork,
		targetAddr:    targetAddr,
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

	l, err := p.mnet.Listen(p.listenNetwork, p.listenAddr)
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
	c2, err := p.mnet.Dial(p.targetNetwork, p.targetAddr)
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
