package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"sync"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

type PortContext struct {
	svcName    string
	listenAddr string
	listener   net.Listener
	target     string
	agent      string

	dial  agent.Dialer
	proxy *socks.ProxyInfo
}

func NewPortContext(svcName, agent, key, val string, dial agent.Dialer, proxy *socks.ProxyInfo) (*PortContext, error) {
	ctx := &PortContext{
		svcName: svcName,
		dial:    dial,
		proxy:   proxy,
		agent:   agent,
		target:  val,
	}
	portstr := key
	isLocal := true
	if portstr[0] == ':' {
		isLocal = false
		portstr = portstr[1:]
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		return nil, err
	}
	if port < 0 || port > 0xffff {
		return nil, errors.New("invalid port number")
	}
	if isLocal {
		ctx.listenAddr = fmt.Sprintf("%v:%v", "localhost", port)
	} else {
		ctx.listenAddr = fmt.Sprintf("%v:%v", "0.0.0.0", port)
	}

	return ctx, nil
}

func (ctx *PortContext) Start(wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	l, err := net.Listen("tcp4", ctx.listenAddr)
	if err != nil {
		log.Printf("[%v] listen %v failed: %v\n", ctx.svcName, ctx.listenAddr, err)
		return
	}

	name := fmt.Sprintf("%v, %v >> %v@%v", ctx.svcName, ctx.listenAddr, ctx.target, ctx.agent)

	runsvc(name, wg, func() {
		for {
			c1, err := l.Accept()
			if err != nil {
				break
			}

			go func(c1 net.Conn) {
				defer c1.Close()
				c2, err := ctx.dial()
				if err != nil {
					log.Printf("[%v] dial '%v' failed: %v\n", ctx.svcName, ctx.agent, err)
					return
				}
				defer c2.Close()

				// 使用socks5升级连接，访问目标
				c2, err = ctx.proxy.Upgrade(c2, ctx.target)
				if err != nil {
					return
				}

				link(c1, c2)
			}(c1)
		}
	})
}

func link(c1, c2 net.Conn) {
	go func() {
		io.Copy(c1, c2)
		c1.Close()
		c2.Close()
	}()

	io.Copy(c2, c1)
	c1.Close()
	c2.Close()
}
func (ctx *PortContext) Close() error {
	return ctx.listener.Close()
}

type QuickVisit struct {
	mnet   *agent.MixNet
	info   agent.ServiceInfo
	ports  []*PortContext
	agent  string
	secret string
}

func NewQuickVisit(mnet *agent.MixNet, info agent.ServiceInfo) *QuickVisit {
	agent := info.Param["agent"]
	secret := info.Param["secret"]

	proxy := &socks.ProxyInfo{
		Network:  "tcp4",
		Address:  "",
		NeedAuth: true,
		Username: "",
		Password: secret,
	}
	vals := url.Values{}
	vals.Add("secret", QuickSecret)
	dial, _ := mnet.URLDialer(fmt.Sprintf("flex://%v:%v?%v", agent, QuickPort, vals.Encode()))

	ports := make([]*PortContext, 0)
	for k, v := range info.Param {
		ctx, err := NewPortContext(info.Name(), agent, k, v, dial, proxy)
		if err == nil {
			ports = append(ports, ctx)
		}
	}

	return &QuickVisit{
		mnet:   mnet,
		info:   info,
		ports:  ports,
		agent:  agent,
		secret: secret,
	}
}

func (p *QuickVisit) Info() string {
	if p.info.Enable {
		u := fmt.Sprintf("flex://%v:%v", p.agent, QuickPort)
		return agent.Green(fmt.Sprintf("%11v %24v %24v", p.info.Type, "multi", u))
	}
	return agent.Yellow(fmt.Sprintf("%11v %24v", p.info.Type, "disabled"))
}

func (p *QuickVisit) Start(wg *sync.WaitGroup) error {
	if !p.info.Enable {
		return errors.New("service disabled")
	}

	for _, port := range p.ports {
		port.Start(wg)
	}
	return nil
}

func (p *QuickVisit) Close() error {
	for _, port := range p.ports {
		port.Close()
	}
	return nil
}
