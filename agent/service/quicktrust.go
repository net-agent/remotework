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
	agent *agent.AgentInfo
	mnet  agent.Network

	closer  io.Closer
	network string
	host    string
	query   string
	users   map[string]string
}

func NewQuickTrust(agt *agent.AgentInfo, mnet agent.Network) *QuickTrust {
	users := make(map[string]string)
	for k, v := range agt.QuickTrust.WhiteList {
		// 这个 dialer 一定是 cipherconn
		users[k+"/secret"] = v
	}
	return &QuickTrust{
		agent:   agt,
		mnet:    mnet,
		network: agt.Network,
		host:    fmt.Sprintf("%v:%v", 0, QuickPort),
		query:   fmt.Sprintf("?secret=%v", QuickSecret),
		users:   users,
	}
}

func (s *QuickTrust) Start(wg *sync.WaitGroup) error {
	if !s.agent.Enable {
		return errors.New("service disabled")
	}
	name := fmt.Sprintf("%v.trust", s.agent.Network)
	l, err := agent.ListenURL(s.mnet, fmt.Sprintf("%v://%v%v", s.network, s.host, s.query))
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
			log.Printf("[%v] empty dialer info\n", name)
			return errAuthFailed
		}

		pswd, found := s.users[u]
		if !found {
			log.Printf("[%v] user='%v' not found\n", name, u)
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

	runsvc(name, wg, func() { svc.Run(l) })
	return nil
}

func (s *QuickTrust) Close() error {
	if !s.agent.QuickTrust.Enable {
		return nil
	}
	c := s.closer
	s.closer = nil
	return c.Close()
}
