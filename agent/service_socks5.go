package agent

import (
	"errors"
	"io"
	"net"

	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/socks"
)

type Socks5Controller struct {
	state *ServiceState
	nl    *utils.NamedLogger
	hub   *Hub

	listener net.Listener
	server   socks.Server
}

func NewSocks5Controller(hub *Hub, state *ServiceState) *Socks5Controller {
	return &Socks5Controller{
		state: state,
		nl:    utils.NewNamedLogger(state.Name, true),
		hub:   hub,
	}
}

func (s *Socks5Controller) Init() error {
	s.server = socks.NewPswdServer(s.state.Username, s.state.Password)
	s.server.SetConnLinker(func(a, b io.ReadWriteCloser) (a2b int64, b2a int64, err error) {
		s.state.AddActiveCount(1)
		defer func() {
			s.state.AddDoneCount(1)
		}()
		return utils.LinkReadWriter(a, b)
	})

	if err := s.Update(); err != nil {
		return err
	}

	return nil
}

func (s *Socks5Controller) Update() error {
	l, err := s.hub.ListenURL(s.state.ListenURL)
	if err != nil {
		return err
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.listener = l

	return nil
}

func (s *Socks5Controller) Start() error {
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

func (s *Socks5Controller) Close() error {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.server != nil {
		s.server.Close()
	}
	return nil
}
