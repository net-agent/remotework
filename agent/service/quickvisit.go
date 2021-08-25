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
	network    string
	domain     string

	dial  agent.QuickDialer
	proxy *socks.ProxyInfo
}

func NewPortContext(hub *agent.NetHub, key, val string) (*PortContext, error) {
	u, err := url.Parse(val)
	if err != nil {
		return nil, err
	}

	network := u.Scheme
	domain := u.User.Username()
	vals := url.Values{}
	vals.Add("secret", QuickSecret)
	dial, _ := hub.URLDialer(fmt.Sprintf("%v://%v:%v?%v", network, domain, QuickPort, vals.Encode()))

	secret, ok := u.User.Password()
	if !ok {
		return nil, errors.New("parse secret failed")
	}
	proxy := &socks.ProxyInfo{
		Network:  "tcp4",
		Address:  "", // 只用到upgrader，不需要创建连接
		NeedAuth: true,
		Username: "", // 由dialer进行进行校验
		Password: secret,
	}

	ctx := &PortContext{
		svcName: fmt.Sprintf("quickvisit/%v", key),
		network: network,
		domain:  domain,
		dial:    dial,
		proxy:   proxy,
		target:  val,
	}

	// 确定实际监听地址（区别：0.0.0.0与localhost）
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
	l, err := net.Listen("tcp4", ctx.listenAddr)
	if err != nil {
		log.Printf("[%v] listen %v failed: %v\n", ctx.svcName, ctx.listenAddr, err)
		return
	}

	name := fmt.Sprintf("%v, %v >> %v@%v/%v", ctx.svcName, ctx.listenAddr, ctx.target, ctx.network, ctx.domain)

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
					log.Printf("[%v] dial '%v/%v' failed: %v\n", ctx.svcName, ctx.network, ctx.domain, err)
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
	hub   *agent.NetHub
	info  agent.ServiceInfo
	ports []*PortContext
}

func NewQuickVisit(hub *agent.NetHub, info agent.ServiceInfo) *QuickVisit {
	ports := make([]*PortContext, 0)
	for k, v := range info.Param {
		ctx, err := NewPortContext(hub, k, v)
		if err == nil {
			ports = append(ports, ctx)
		}
	}

	return &QuickVisit{
		hub:   hub,
		info:  info,
		ports: ports,
	}
}

func (p *QuickVisit) Info() string {
	if p.info.Enable {
		return agent.Green(fmt.Sprintf("%11v %24v %24v", p.info.Type, "enabled", ""))
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
