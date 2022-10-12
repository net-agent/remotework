package agent

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"

	"github.com/net-agent/remotework/utils"
)

type Portproxy struct {
	svcinfo
	nl        *utils.NamedLogger
	hub       *Hub
	listenURL string
	targetURL string
	logName   string

	listener net.Listener
	dialer   QuickDialer
	mut      sync.Mutex

	listenNetwork string
}

func NewPortproxyWithConfig(hub *Hub, info PortproxyInfo) *Portproxy {
	return NewPortproxy(hub, info.ListenURL, info.TargetURL, info.LogName)
}

func NewPortproxy(hub *Hub, listenURL, targetURL, logName string) *Portproxy {
	return &Portproxy{
		nl:        utils.NewNamedLogger(logName, true),
		hub:       hub,
		listenURL: listenURL,
		targetURL: targetURL,
		logName:   logName,
		svcinfo: svcinfo{
			name:    svcName(logName, "portproxy"),
			svctype: "portproxy",
			listen:  listenURL,
			target:  targetURL,
		},
	}
}

func NewRDPWithConfig(hub *Hub, info RDPInfo) *Portproxy {
	return NewPortproxy(hub, info.ListenURL, fmt.Sprintf("tcp://localhost:%v", utils.GetRDPPort()), info.LogName)
}
func NewRDP(hub *Hub, listenURL, logName string) *Portproxy {
	return NewPortproxy(hub, listenURL, fmt.Sprintf("tcp://localhost:%v", utils.GetRDPPort()), logName)
}

func (s *Portproxy) Network() string { return s.listenNetwork }

func (s *Portproxy) Init() (reterr error) {
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
	p.AddActiveCount(1)
	defer func() {
		c1.Close()
		p.AddDoneCount(1)
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
