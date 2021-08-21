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

const (
	QuickPort   = 71
	QuickSecret = "qu1ckxTru5t"
)

type QuickTrust struct {
	hub  *agent.NetHub
	info agent.ServiceInfo

	closer io.Closer
	listen string
	users  map[string]string
}

func NewQuickTrust(hub *agent.NetHub, info agent.ServiceInfo) *QuickTrust {
	users := make(map[string]string)
	for k, v := range info.Param {
		// 这个 dialer 一定是 cipherconn
		users[k+"/secret"] = v
	}
	return &QuickTrust{
		hub:    hub,
		info:   info,
		listen: fmt.Sprintf("flex://0:%v?secret=%v", QuickPort, QuickSecret),
		users:  users,
	}
}

func (s *QuickTrust) Info() string {
	if s.info.Enable {
		u := fmt.Sprintf("flex://0:%v", QuickPort)
		return agent.Green(fmt.Sprintf("%11v %24v %24v", s.info.Type, u, "tcp4"))
	}
	return agent.Yellow(fmt.Sprintf("%11v %24v", s.info.Type, "disabled"))
}

func (s *QuickTrust) Start(wg *sync.WaitGroup) error {
	if !s.info.Enable {
		return errors.New("service disabled")
	}

	l, err := s.hub.ListenURL(s.listen)
	if err != nil {
		return err
	}

	errAuthFailed := errors.New("auth failed")
	checker := socks.PswdAuthChecker(func(u, p string, ctx socks.Context) error {
		conn := ctx.GetConn()

		// 使用 packet.Stream 的 Dialer 接口，获取请求来自于谁
		d, ok := conn.(interface{ Dialer() string })
		if ok {
			u = d.Dialer()
		}
		if u == "" {
			log.Printf("[%v] empty dialer info\n", s.info.Type)
			return errAuthFailed
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

	runsvc(s.info.Name(), wg, func() { svc.Run(l) })
	return nil
}

func (s *QuickTrust) Close() error {
	if !s.info.Enable {
		return nil
	}
	c := s.closer
	s.closer = nil
	return c.Close()
}
