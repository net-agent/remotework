package vservice

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/net-agent/remotework/utils"
)

type PortproxyController struct {
	state    *ServiceState
	log      *slog.Logger
	lf       ListenerFactory
	df       DialerFactory
	listener net.Listener
	dialer   QuickDialer
}

func NewPortproxyController(lf ListenerFactory, df DialerFactory, state *ServiceState) *PortproxyController {
	return &PortproxyController{
		state: state,
		log:   utils.NewModuleLogger("svc." + state.Name),
		lf:    lf,
		df:    df,
	}
}

func (s *PortproxyController) Init() error {
	dialer, err := s.df.URLDialer(s.state.TargetURL)
	if err != nil {
		return fmt.Errorf("parse target url failed: %v", err)
	}
	s.dialer = dialer

	s.listener, err = s.lf.ListenURL(s.state.ListenURL)
	if err != nil {
		return fmt.Errorf("listen url failed: %v", err)
	}
	return nil
}

func (p *PortproxyController) Start() error {
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

func (p *PortproxyController) Close() error {
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}

func (p *PortproxyController) serve(dialConn net.Conn) {
	p.state.AddActiveCount(1)
	defer func() {
		dialConn.Close()
		p.state.AddDoneCount(1)
	}()

	targetConn, err := p.dialer()
	if err != nil {
		p.log.Error("dial error", "target", p.state.TargetURL, "err", err)
		return
	}
	defer targetConn.Close()

	dialer := GetRemoteInfo(dialConn)
	start := time.Now()

	p.log.Info("pipe created", "from", dialer, "to", p.state.TargetURL)
	dialRecv, dialSent, _ := utils.RelayConns(dialConn, targetConn)

	lifetimeInfo := fmt.Sprintf("sent=%v, recv=%v, lifetime=%v",
		humanize.IBytes(uint64(dialSent)), humanize.IBytes(uint64(dialRecv)), time.Since(start).Round(time.Second))

	p.log.Info("pipe stopped", "from", dialer, "to", p.state.TargetURL, "stats", lifetimeInfo)
}
