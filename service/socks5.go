package service

import (
	"errors"
	"log"
	"net"
	"net/url"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/socks"
)

type Socks5 struct {
	hub       *agent.NetHub
	listenURL string
	username  string
	password  string
	logName   string

	listener      net.Listener
	listenNetwork string
	server        socks.Server
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
func (s *Socks5) Network() string { return s.listenNetwork }

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
	s.server = socks.NewPswdServer(s.username, s.password)

	u, err := url.Parse(s.listenURL)
	if err != nil {
		return err
	}
	s.listenNetwork = u.Scheme

	if err := s.Update(); err != nil {
		return err
	}

	return nil
}

func (s *Socks5) Update() error {
	l, err := s.hub.ListenURL(s.listenURL)
	if err != nil {
		return err
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.listener = l

	return nil
}

func (s *Socks5) Start() error {
	if s.server == nil || s.listener == nil {
		return errors.New("init failed")
	}

	l := s.listener
	for {
		err := s.server.Run(l)

		if l != s.listener && s.listener != nil {
			log.Printf("[%v] listener updated\n", s.logName)
			l = s.listener
			continue
		}

		return err
	}
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
