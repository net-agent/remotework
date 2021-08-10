package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

type QuickVisit struct {
	mnet    *agent.MixNet
	info    agent.ServiceInfo
	closers []io.Closer

	agent  string
	secret string
	ports  map[string]string
}

func NewQuickVisit(mnet *agent.MixNet, info agent.ServiceInfo) *QuickVisit {
	ports := make(map[string]string)
	for k, v := range info.Param {
		if k[0] == ':' {
			ports["0.0.0.0"+k] = v
		} else if k[0] >= '0' && k[0] <= '9' {
			ports["localhost:"+k] = v
		}
	}

	log.Println("ports:", ports)

	return &QuickVisit{
		mnet: mnet,
		info: info,

		agent:  info.Param["agent"],
		secret: info.Param["secret"],
		ports:  ports,
	}
}

func (p *QuickVisit) Info() string {
	if p.info.Enable {
		return agent.Green(fmt.Sprintf("%11v %24v %24v", p.info.Type, "multi", p.agent))
	}
	return agent.Yellow(fmt.Sprintf("%11v %24v", p.info.Type, "disabled"))
}

func (p *QuickVisit) Run() error {
	if !p.info.Enable {
		return errors.New("service disabled")
	}
	defer log.Printf("[%v] stopped.\n", p.info.Type)

	var wg sync.WaitGroup
	for port, target := range p.ports {
		wg.Add(1)
		go func(addr, target string) {
			defer wg.Done()

			l, err := net.Listen("tcp4", addr)
			if err != nil {
				log.Printf("[%v] listen failed. addr='%v' err='%v'\n", p.info.Type, addr, err)
				return
			}
			p.closers = append(p.closers, l)

			u := fmt.Sprintf("flex://%v:70?secret=quick", p.agent)
			targetDialer, err := p.mnet.URLDialer(u)
			if err != nil {
				log.Printf("[%v] invalid url. url='%v' err='%v'\n", p.info.Type, u, err)
				return
			}

			proxy := socks.ProxyInfo{
				Network:  "tcp4",
				Address:  "",
				NeedAuth: true,
				Username: "",
				Password: p.secret,
			}

			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}

				go func(c1 net.Conn) {
					c2, err := targetDialer()
					if err != nil {
						c1.Close()
						return
					}

					c2, err = proxy.Upgrade(c2, target)
					if err != nil {
						c1.Close()
						c2.Close()
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
				}(conn)
			}
		}(port, target)
	}

	wg.Wait()
	return nil
}

func (p *QuickVisit) Close() error {
	for _, closer := range p.closers {
		closer.Close()
	}
	return nil
}
