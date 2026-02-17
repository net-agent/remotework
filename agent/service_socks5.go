package agent

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/socks"
)

type Socks5Controller struct {
	state *ServiceState
	nl    *utils.NamedLogger
	hub   *Hub

	mut      sync.Mutex
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
		dialer := getRemoteInfo(a)
		start := time.Now()
		s.state.AddActiveCount(1)
		defer func() {
			s.state.AddDoneCount(1)
			a.Close()
			b.Close()
			s.nl.Printf("link stopped, from='%v', alive=%v\n", dialer, time.Since(start).Round(time.Second))
		}()
		s.nl.Printf("link created, from='%v'\n", dialer)
		return utils.LinkReadWriteCloser(a, b)
	})

	if err := s.Update(); err != nil {
		return err
	}

	return nil
}

func (s *Socks5Controller) Update() error {
	s.mut.Lock()
	defer s.mut.Unlock()

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

func (s *Socks5Controller) getListener() net.Listener {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.listener
}

func (s *Socks5Controller) Start() error {
	if s.server == nil || s.getListener() == nil {
		return errors.New("init failed")
	}

	l := s.getListener()
	for {
		err := s.server.Run(l)

		newListener := s.getListener()
		if newListener != nil && l != newListener {
			s.nl.Println("listener updated")
			l = newListener
			continue
		}

		return err
	}
}

func (s *Socks5Controller) Close() error {
	s.mut.Lock()
	defer s.mut.Unlock()

	if s.listener != nil {
		s.listener.Close()
	}
	if s.server != nil {
		s.server.Close()
	}
	return nil
}
