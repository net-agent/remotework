package service

import (
	"errors"
	"net"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

type Socks5 struct {
	hub       *agent.NetHub
	listenURL string
	username  string
	password  string
	logName   string

	listener net.Listener
	server   socks.Server
}

func NewSocks5(hub *agent.NetHub, listenURL, username, password, logName string) *Socks5 {
	return &Socks5{
		hub:       hub,
		listenURL: listenURL,
		username:  username,
		password:  password,
		logName:   logName,
	}
}

func (s *Socks5) Name() string {
	if s.logName != "" {
		return s.logName
	}
	return "sock5"
}

func (s *Socks5) Report() agent.ReportInfo {
	return agent.ReportInfo{
		Name:    s.Name(),
		State:   "uninit",
		Listen:  s.listenURL,
		Target:  "-",
		Actives: 0,
	}
}

func (s *Socks5) Init() error {
	var err error
	s.listener, err = s.hub.ListenURL(s.listenURL)
	if err != nil {
		return err
	}

	s.server = socks.NewPswdServer(s.username, s.password)

	return nil
}

func (s *Socks5) Start() error {
	if s.server == nil || s.listener == nil {
		return errors.New("init failed")
	}

	return s.server.Run(s.listener)
}

func (s *Socks5) Close() error {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.server != nil {
		s.server.Close()
	}
	return nil
}
