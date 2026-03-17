package vservice

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/socks"
)

type Socks5Controller struct {
	state    *ServiceState
	log      *slog.Logger
	lf       ListenerFactory
	listener net.Listener
	server   socks.Server
}

func NewSocks5Controller(lf ListenerFactory, state *ServiceState) *Socks5Controller {
	return &Socks5Controller{
		state: state,
		log:   utils.NewModuleLogger("svc." + state.Name),
		lf:    lf,
	}
}

func (s *Socks5Controller) Init() error {
	s.server = socks.NewPswdServer(s.state.Username, s.state.Password)
	s.server.SetConnLinker(func(a, b io.ReadWriteCloser) (a2b int64, b2a int64, err error) {
		dialer := GetRemoteInfo(a)
		start := time.Now()
		s.state.AddActiveCount(1)
		defer func() {
			s.state.AddDoneCount(1)
			a.Close()
			b.Close()
			s.log.Info("link stopped", "from", dialer, "alive", time.Since(start).Round(time.Second))
		}()
		s.log.Info("link created", "from", dialer)
		return utils.RelayConns(a, b)
	})

	var err error
	s.listener, err = s.lf.ListenURL(s.state.ListenURL)
	if err != nil {
		return err
	}
	return nil
}

func (s *Socks5Controller) Start() error {
	if s.server == nil || s.listener == nil {
		return errors.New("init failed")
	}
	return s.server.Run(s.listener)
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
