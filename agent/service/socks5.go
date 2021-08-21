package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

type Socks5 struct {
	hub  *agent.NetHub
	info agent.ServiceInfo

	closer   io.Closer
	listen   string
	username string
	password string
}

func NewSocks5(hub *agent.NetHub, info agent.ServiceInfo) *Socks5 {
	return &Socks5{
		hub:      hub,
		info:     info,
		listen:   info.Param["listen"],
		username: info.Param["username"],
		password: info.Param["password"],
	}
}

func (s *Socks5) Info() string {
	if s.info.Enable {
		return agent.Green(fmt.Sprintf("%11v %24v %24v", s.info.Type, s.listen, "tcp4"))
	}
	return agent.Yellow(fmt.Sprintf("%11v %24v", s.info.Type, "disabled"))
}

func (s *Socks5) Start(wg *sync.WaitGroup) error {
	if !s.info.Enable {
		return errors.New("service disabled")
	}

	l, err := s.hub.ListenURL(s.info.Param["listen"])
	if err != nil {
		return err
	}

	svc := socks.NewPswdServer(s.username, s.password)
	s.closer = svc
	runsvc(s.info.Name(), wg, func() {
		err := svc.Run(l)
		if err != nil {
			log.Printf("[%v] exit. err=%v\n", s.info.Name(), err)
		}
	})
	return nil
}

func (s *Socks5) Close() error {
	if !s.info.Enable {
		return nil
	}
	c := s.closer
	s.closer = nil
	return c.Close()
}
