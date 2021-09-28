package service

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

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
	logName string

	users    map[string]string
	svc      socks.Server
	listener net.Listener
	mut      sync.Mutex

	actives int32
	dones   int32
}

func NewQuickTrust(hub *agent.NetHub, network string, domains map[string]string, logName string) *QuickTrust {
	return &QuickTrust{
		hub:     hub,
		network: network,
		domains: domains,
		logName: logName,
	}
}

func (s *QuickTrust) Name() string {
	if s.logName != "" {
		return s.logName
	}
	return "trust"
}
func (s *QuickTrust) Network() string { return s.network }
func (s *QuickTrust) Report() agent.ReportInfo {
	return agent.ReportInfo{
		Name:    s.Name(),
		State:   "uninit",
		Listen:  "-",
		Target:  "-",
		Actives: s.actives,
		Dones:   s.dones,
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
	if err := s.Update(); err != nil {
		return err
	}

	return nil
}

func (s *QuickTrust) Update() error {
	s.mut.Lock()
	defer s.mut.Unlock()

	listenURL := fmt.Sprintf("%v://0:%v?secret=%v", s.network, QuickPort, QuickSecret)
	l, err := s.hub.ListenURL(listenURL)
	if err != nil {
		return err
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.listener = l

	return nil
}

func (s *QuickTrust) Start() error {
	if s.svc == nil || s.listener == nil {
		return errors.New("init failed")
	}
	l := s.listener
	for {
		err := s.svc.Run(l)

		if l != s.listener && s.listener != nil {
			log.Printf("[%v] listener updated\n", s.logName)
			l = s.listener
			continue
		}

		return err
	}
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
