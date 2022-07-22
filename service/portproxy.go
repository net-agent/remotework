package service

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

type Portproxy struct {
	nl        *utils.NamedLogger
	hub       *agent.NetHub
	listenURL string
	targetURL string
	logName   string

	listener net.Listener
	dialer   agent.QuickDialer
	mut      sync.Mutex

	listenNetwork string
	actives       int32
	dones         int32
}

func NewPortproxy(hub *agent.NetHub, listenURL, targetURL, logName string) *Portproxy {
	return &Portproxy{
		nl:        utils.NewNamedLogger(logName, true),
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

func (s *Portproxy) Network() string { return s.listenNetwork }

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

	if err = s.Update(); err != nil {
		return err
	}

	return nil
}

func (s *Portproxy) Update() error {
	s.mut.Lock()
	defer s.mut.Unlock()

	l, err := s.hub.ListenURL(s.listenURL)
	if err != nil {
		return fmt.Errorf("listen url failed: %v", err)
	}

	// close old listener
	if s.listener != nil {
		s.listener.Close()
	}
	s.listener = l

	return nil
}

func (s *Portproxy) getlistener() net.Listener {
	s.mut.Lock()
	defer s.mut.Unlock()

	return s.listener
}

func (p *Portproxy) Start() error {
	if p.dialer == nil || p.listener == nil {
		return errors.New("init failed")
	}

	l := p.getlistener()

	for {
		conn, err := l.Accept()

		if err == nil {
			go p.serve(conn)
			continue
		}

		//
		// accept连接出现错误后，尝试恢复服务，等待新的listener
		// 如果尝试恢复listener失败后，才真正返回错误
		//
		newListener := p.getlistener()
		if newListener != nil && l != newListener {
			// 更新listener成功，继续恢复accept循环
			l = newListener

			p.nl.Println("listener updated")

			continue
		}

		// 最终恢复失败后，返回
		return err
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
		p.nl.Printf("dial error. target=%v, err=%v\n", p.targetURL, err)
		return
	}

	p.nl.Printf("linked. %v > %v > %v\n", dialer, p.listenURL, p.targetURL)
	utils.LinkReadWriter(c1, c2)
}
