package service

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"sync/atomic"

	"github.com/net-agent/remotework/agent"
)

type Portproxy struct {
	hub       *agent.NetHub
	listenURL string
	targetURL string
	logName   string

	enableLog bool
	listener  net.Listener
	dialer    agent.QuickDialer

	listenNetwork string
	actives       int32
	dones         int32
}

func NewPortproxy(hub *agent.NetHub, listenURL, targetURL, logName string) *Portproxy {
	return &Portproxy{
		hub:       hub,
		listenURL: listenURL,
		targetURL: targetURL,
		logName:   logName,
	}
}

func NewRDP(hub *agent.NetHub, listenURL, logName string) *Portproxy {
	return NewPortproxy(hub, listenURL, fmt.Sprintf("tcp://localhost:%v", rdpPortNumber()), logName)
}

func (s *Portproxy) Name() string {
	if s.logName != "" {
		return s.logName
	}
	return "portp"
}

func (s *Portproxy) Report() agent.ReportInfo {
	return agent.ReportInfo{
		Name:    s.Name(),
		State:   "uninit",
		Listen:  s.listenURL,
		Target:  s.targetURL,
		Actives: s.actives,
		Dones:   s.dones,
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

	s.enableLog = s.logName != ""

	return nil
}

func (p *Portproxy) Start() error {
	if p.dialer == nil || p.listener == nil {
		return errors.New("init failed")
	}

	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return err
		}
		go p.serve(conn)
	}
}

func (p *Portproxy) Close() error {
	return p.listener.Close()
}

func (p *Portproxy) serve(c1 net.Conn) {
	atomic.AddInt32(&p.actives, 1)
	defer func() {
		c1.Close()
		atomic.AddInt32(&p.actives, -1)
		atomic.AddInt32(&p.dones, 1)
	}()

	var dialer string
	if s, ok := c1.(interface{ Dialer() string }); ok {
		dialer = p.listenNetwork + "://" + s.Dialer()
	} else {
		dialer = "tcp://" + c1.RemoteAddr().String()
	}

	c2, err := p.dialer()
	if err != nil {
		log.Printf("[%v] dial error. target=%v, err=%v\n", p.logName, p.targetURL, err)
		return
	}

	if p.enableLog {
		log.Printf("[%v] linked. %v > %v > %v\n", p.logName, dialer, p.listenURL, p.targetURL)
	}
	link(c1, c2)
}
