package service

import (
	"errors"
	"fmt"
	"io"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

type Socks5 struct {
	mnet *agent.MixNet
	info agent.ServiceInfo

	closer   io.Closer
	listen   string
	username string
	password string
}

func NewSocks5(mnet *agent.MixNet, info agent.ServiceInfo) *Socks5 {
	return &Socks5{
		mnet:     mnet,
		info:     info,
		listen:   info.Param["listen"],
		username: info.Param["username"],
		password: info.Param["password"],
	}
}

func (s *Socks5) Info() string {
	if s.info.Enable {
		return green(fmt.Sprintf("%11v %24v %24v", s.info.Type, s.listen, s.username))
	}
	return yellow(fmt.Sprintf("%11v %24v", s.info.Type, "disabled"))
}

func (s *Socks5) Run() error {
	if !s.info.Enable {
		return errors.New("service disabled")
	}

	network, addr, err := ParseAddr(s.info.Param["listen"])
	if err != nil {
		return err
	}

	l, err := s.mnet.Listen(network, addr)
	if err != nil {
		return err
	}

	svc := socks.NewPswdServer(s.username, s.password)
	s.closer = svc
	return svc.Run(l)
}

func (s *Socks5) Close() error {
	if !s.info.Enable {
		return nil
	}
	c := s.closer
	s.closer = nil
	return c.Close()
}
