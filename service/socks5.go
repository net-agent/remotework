package service

import (
	"errors"
	"io"
	"net"
	"net/url"
	"sync/atomic"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/socks"
)

type Socks5 struct {
	nl        *utils.NamedLogger
	hub       *agent.NetHub
	listenURL string
	username  string
	password  string
	logName   string

	listener      net.Listener
	listenNetwork string
	server        socks.Server

	actives int32
	dones   int32
	state   string
}

func NewSocks5(hub *agent.NetHub, listenURL, username, password, logName string) *Socks5 {
	return &Socks5{
		nl:        utils.NewNamedLogger(logName, true),
		hub:       hub,
		listenURL: listenURL,
		username:  username,
		password:  password,
		logName:   logName,
	}
}

func (s *Socks5) SetState(st string) { s.state = st }
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
		State:   s.state,
		Listen:  s.listenURL,
		Target:  "-",
		Actives: s.actives,
		Dones:   s.dones,
	}
}

func (s *Socks5) Init() error {
	s.server = socks.NewPswdServer(s.username, s.password)
	s.server.SetConnLinker(func(a, b io.ReadWriteCloser) (a2b int64, b2a int64, err error) {
		atomic.AddInt32(&s.actives, 1)
		defer func() {
			atomic.AddInt32(&s.actives, -1)
			atomic.AddInt32(&s.dones, 1)
		}()
		return utils.LinkReadWriter(a, b)
	})

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
			s.nl.Println("listener updated")
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
