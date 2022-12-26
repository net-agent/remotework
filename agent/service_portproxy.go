package agent

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/net-agent/remotework/utils"
)

type PortproxyController struct {
	state *ServiceState
	nl    *utils.NamedLogger
	hub   *Hub

	listener net.Listener
	dialer   QuickDialer
	mut      sync.Mutex
}

func NewPortproxyController(hub *Hub, state *ServiceState) *PortproxyController {
	return &PortproxyController{
		state: state,
		nl:    utils.NewNamedLogger(state.Name, true),
		hub:   hub,
	}
}

func (s *PortproxyController) Init() (reterr error) {
	dialer, err := s.hub.URLDialer(s.state.TargetURL)
	if err != nil {
		return fmt.Errorf("parse target url failed: %v", err)
	}
	s.dialer = dialer

	if err = s.Update(); err != nil {
		return err
	}

	return nil
}

func (s *PortproxyController) Update() error {
	s.mut.Lock()
	defer s.mut.Unlock()

	l, err := s.hub.ListenURL(s.state.ListenURL)
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

func (s *PortproxyController) getlistener() net.Listener {
	s.mut.Lock()
	defer s.mut.Unlock()

	return s.listener
}

func (p *PortproxyController) Start() error {
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

func (p *PortproxyController) Close() error {
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}

func (p *PortproxyController) serve(c1 net.Conn) {
	p.state.AddActiveCount(1)
	defer func() {
		c1.Close()
		p.state.AddDoneCount(1)
	}()

	var dialer string
	if s, ok := c1.(interface{ Dialer() string }); ok {
		dialer = "mnet://" + s.Dialer()
	} else {
		dialer = "tcp://" + c1.RemoteAddr().String()
	}

	c2, err := p.dialer()
	if err != nil {
		p.nl.Printf("dial error. target=%v, err=%v\n", p.state.TargetURL, err)
		return
	}

	p.nl.Printf("linked. %v > %v > %v\n", dialer, p.state.ListenURL, p.state.TargetURL)
	utils.LinkReadWriter(c1, c2)
}
