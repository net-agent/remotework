package service

import (
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

type QuickTrust struct {
	mnet *agent.MixNet
	info agent.ServiceInfo

	closer io.Closer
	listen string
	users  map[string]string
}

func NewQuickTrust(mnet *agent.MixNet, info agent.ServiceInfo) *QuickTrust {
	return &QuickTrust{
		mnet:   mnet,
		info:   info,
		listen: "flex://0:70?secret=quick",
		users:  info.Param,
	}
}

func (s *QuickTrust) Info() string {
	if s.info.Enable {
		return agent.Green(fmt.Sprintf("%11v %24v %24v", s.info.Type, s.listen, "quick"))
	}
	return agent.Yellow(fmt.Sprintf("%11v %24v", s.info.Type, "disabled"))
}

func (s *QuickTrust) Run() error {
	if !s.info.Enable {
		return errors.New("service disabled")
	}
	defer log.Printf("[%v] stopped.\n", s.info.Type)

	l, err := s.mnet.ListenURL(s.listen)
	if err != nil {
		return err
	}

	errAuthFailed := errors.New("auth failed")
	checker := socks.PswdAuthChecker(func(u, p string, ctx socks.Context) error {
		conn := ctx.GetConn()
		d, ok := conn.(interface{ Dialer() string })
		if ok {
			u = d.Dialer()
		}

		pswd, found := s.users[u]
		if !found {
			log.Printf("[%v] user='%v' not found\n", s.info.Type, u)
			return errAuthFailed
		}
		if pswd != p {
			return errAuthFailed
		}
		return nil
	})
	svc := socks.NewServer()
	svc.SetAuthChecker(checker)
	s.closer = svc
	return svc.Run(l)
}

func (s *QuickTrust) Close() error {
	if !s.info.Enable {
		return nil
	}
	c := s.closer
	s.closer = nil
	return c.Close()
}
