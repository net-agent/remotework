package agent

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
	state *ServiceState
	log   *slog.Logger
	lf    ListenerFactory
	df    DialerFactory

	hsl    *HotSwapListener
	dialer QuickDialer
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

	s.hsl = NewHotSwapListener(func() (net.Listener, error) {
		return s.lf.ListenURL(s.state.ListenURL)
	})

	if err = s.hsl.Refresh(); err != nil {
		return fmt.Errorf("listen url failed: %v", err)
	}

	return nil
}

func (s *PortproxyController) Update() error {
	return s.hsl.Refresh()
}

func (p *PortproxyController) Start() error {
	if p.dialer == nil || p.hsl == nil {
		return errors.New("init failed")
	}

	l := p.hsl.Get()

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
		time.Sleep(100 * time.Millisecond) // 等待Update()有机会替换listener
		newListener := p.hsl.Get()
		if newListener != nil && l != newListener {
			// 更新listener成功，继续恢复accept循环
			l = newListener

			p.log.Info("listener updated")

			continue
		}

		// 最终恢复失败后，返回
		return err
	}
}

func (p *PortproxyController) Close() error {
	if p.hsl != nil {
		return p.hsl.Close()
	}
	return nil
}

func (p *PortproxyController) serve(dialConn net.Conn) {
	p.state.AddActiveCount(1)
	defer func() {
		dialConn.Close()
		p.state.AddDoneCount(1)
	}()

	targetConn, err := p.dialer() // quick dial target
	if err != nil {
		p.log.Error("dial error", "target", p.state.TargetURL, "err", err)
		return
	}
	defer targetConn.Close()

	dialer := getRemoteInfo(dialConn)
	start := time.Now()

	p.log.Info("pipe created", "from", dialer, "to", p.state.TargetURL)
	dialRecv, dialSent, _ := utils.LinkReadWriteCloser(dialConn, targetConn)

	lifetimeInfo := fmt.Sprintf("sent=%v, recv=%v, lifetime=%v",
		humanize.IBytes(uint64(dialSent)), humanize.IBytes(uint64(dialRecv)), time.Since(start).Round(time.Second))

	p.log.Info("pipe stopped", "from", dialer, "to", p.state.TargetURL, "stats", lifetimeInfo)
}
