package service

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"

	"github.com/net-agent/remotework/agent"
)

type Portproxy struct {
	hub       *agent.NetHub
	listenURL string
	targetURL string
	enableLog bool

	listener net.Listener
	dialer   agent.QuickDialer

	listenNetwork string
}

func NewPortproxy(hub *agent.NetHub, listenURL, targetURL string) *Portproxy {
	return &Portproxy{
		hub:       hub,
		listenURL: listenURL,
		targetURL: targetURL,
	}
}

func (s *Portproxy) Init() error {
	dialer, err := s.hub.URLDialer(s.targetURL)
	if err != nil {
		return fmt.Errorf("parse target url failed: %v", err)
	}
	s.dialer = dialer

	u, err := url.Parse(s.listenURL)
	if err != nil {
		return fmt.Errorf("parse listen url failed: %v", err)
	}
	s.listenNetwork = u.Scheme

	l, err := s.hub.ListenURL(s.listenURL)
	if err != nil {
		return fmt.Errorf("listen url failed: %v", err)
	}
	s.listener = l

	return nil
}

func (p *Portproxy) Start() error {
	if p.dialer == nil || p.listener == nil {
		return errors.New("init failed")
	}

	p.hub.Attach("portproxy", func(hub *agent.NetHub) {
		for {
			conn, err := p.listener.Accept()
			if err != nil {
				return
			}
			go p.serve(conn)
		}
	})
	return nil
}

func (p *Portproxy) Close() error {
	return p.listener.Close()
}

func (p *Portproxy) serve(c1 net.Conn) {
	var dialer string
	if s, ok := c1.(interface{ Dialer() string }); ok {
		dialer = p.listenNetwork + "://" + s.Dialer()
	} else {
		dialer = "tcp://" + c1.RemoteAddr().String()
	}

	c2, err := p.dialer()
	if err != nil {
		log.Printf("[portproxy] dial error. target=%v, err=%v\n", p.targetURL, err)
		c1.Close()
		return
	}

	if p.enableLog {
		log.Printf("[portproxy] linked. %v > %v > %v\n", dialer, p.listenURL, p.targetURL)
	}
	link(c1, c2)
}
