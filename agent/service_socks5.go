package agent

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
	state *ServiceState
	log   *slog.Logger
	lf    ListenerFactory

	hsl    *HotSwapListener
	server socks.Server
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
		dialer := getRemoteInfo(a)
		start := time.Now()
		s.state.AddActiveCount(1)
		defer func() {
			s.state.AddDoneCount(1)
			a.Close()
			b.Close()
			s.log.Info("link stopped", "from", dialer, "alive", time.Since(start).Round(time.Second))
		}()
		s.log.Info("link created", "from", dialer)
		return utils.LinkReadWriteCloser(a, b)
	})

	s.hsl = NewHotSwapListener(func() (net.Listener, error) {
		return s.lf.ListenURL(s.state.ListenURL)
	})

	if err := s.hsl.Refresh(); err != nil {
		return err
	}

	return nil
}

func (s *Socks5Controller) Update() error {
	return s.hsl.Refresh()
}

func (s *Socks5Controller) Start() error {
	if s.server == nil || s.hsl == nil {
		return errors.New("init failed")
	}

	l := s.hsl.Get()
	for {
		err := s.server.Run(l)

		time.Sleep(100 * time.Millisecond) // 等待Update()有机会替换listener
		newListener := s.hsl.Get()
		if newListener != nil && l != newListener {
			s.log.Info("listener updated")
			l = newListener
			continue
		}

		return err
	}
}

func (s *Socks5Controller) Close() error {
	if s.hsl != nil {
		s.hsl.Close()
	}
	if s.server != nil {
		s.server.Close()
	}
	return nil
}
