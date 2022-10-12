package agent

import (
	"errors"
	"io"
	"net"
	"net/url"

	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/socks"
)

type Socks5 struct {
	svcinfo
	nl        *utils.NamedLogger
	hub       *Hub
	listenURL string
	username  string
	password  string
	logName   string

	listener      net.Listener
	listenNetwork string
	server        socks.Server
}

func NewSocks5WithConfig(hub *Hub, info Socks5Info) *Socks5 {
	return NewSocks5(hub, info.ListenURL, info.Username, info.Password, info.LogName)
}

func NewSocks5(hub *Hub, listenURL, username, password, logName string) *Socks5 {
	return &Socks5{
		nl:        utils.NewNamedLogger(logName, true),
		hub:       hub,
		listenURL: listenURL,
		username:  username,
		password:  password,
		logName:   logName,
		svcinfo: svcinfo{
			name:    svcName(logName, "socks5"),
			svctype: "socks5",
			listen:  listenURL,
			target:  "-",
		},
	}
}

func (s *Socks5) Network() string { return s.listenNetwork }
func (s *Socks5) Init() error {
	s.server = socks.NewPswdServer(s.username, s.password)
	s.server.SetConnLinker(func(a, b io.ReadWriteCloser) (a2b int64, b2a int64, err error) {
		s.AddActiveCount(1)
		defer func() {
			s.AddDoneCount(1)
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
