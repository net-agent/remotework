package service

import (
	"errors"
	"fmt"
	"net"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

const (
	QuickPort   = 71
	QuickSecret = "qu1ckxTru5t"
)

type QuickTrust struct {
	hub     *agent.NetHub
	network string
	domains map[string]string

	users    map[string]string
	svc      socks.Server
	listener net.Listener
}

func NewQuickT(hub *agent.NetHub, network string, domains map[string]string) *QuickTrust {
	return &QuickTrust{
		hub:     hub,
		network: network,
		domains: domains,
	}
}

func (s *QuickTrust) Init() error {
	// 初始化users信息
	users := make(map[string]string)
	for k, v := range s.domains {
		users[k+"/secret"] = v
	}

	// 构建socks5 checker
	errAuthFailed := errors.New("auth failed")
	pswdchecker := socks.PswdAuthChecker(func(u, p string, ctx socks.Context) error {
		conn := ctx.GetConn()
		// 使用 packet.Stream 的 Dialer 接口，获取请求的真实身份
		d, ok := conn.(interface{ Dialer() string })
		if ok {
			u = d.Dialer()
		}
		if u == "" {
			return errAuthFailed
		}

		pswd, found := s.users[u]
		if !found {
			return errAuthFailed
		}
		if pswd != p {
			return errAuthFailed
		}
		return nil
	})
	s.svc = socks.NewServer()
	s.svc.SetAuthChecker(pswdchecker)

	// try to listen
	listenURL := fmt.Sprintf("%v://0:%v?secret=%v", s.network, QuickPort, QuickSecret)
	l, err := s.hub.ListenURL(listenURL)
	if err != nil {
		return err
	}
	s.listener = l

	return nil
}

func (s *QuickTrust) Start() error {
	if s.svc == nil || s.listener == nil {
		return errors.New("init failed")
	}
	s.hub.Attach("trust", func(hub *agent.NetHub) {
		s.svc.Run(s.listener)
	})
	return nil
}

func (s *QuickTrust) Close() error {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.svc != nil {
		s.svc.Close()
	}

	return nil
}
