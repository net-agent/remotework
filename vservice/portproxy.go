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

// NewPortproxyController 创建端口转发控制器。
// log 为 nil 时回退到按服务名自建的模块日志器。
func NewPortproxyController(lf ListenerFactory, df DialerFactory, state *ServiceState, log *slog.Logger) *PortproxyController {
	if log == nil {
		log = utils.NewModuleLogger("svc." + state.Name)
	}
	return &PortproxyController{
		state: state,
		log:   log,
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

	// 包装前先取对端信息：登记包装会遮蔽 GetRemoteInfo 的 *stream.Stream 断言
	dialer := GetRemoteInfo(dialConn)
	// 登记 listen 侧连接归属（网络别名取自 ListenURL 的 scheme），供控制面反查；
	// 无法提取连接编号时原样返回，不影响数据通路
	dialConn = p.state.TrackConn(networkOfURL(p.state.ListenURL), dialConn)

	targetConn, err := p.dialer()
	if err != nil {
		p.log.Error("dial error", "target", utils.RedactURL(p.state.TargetURL), "err", err)
		return
	}
	// 登记 dial 侧连接归属（网络别名取自 TargetURL 的 scheme）
	targetConn = p.state.TrackConn(networkOfURL(p.state.TargetURL), targetConn)
	defer targetConn.Close()

	// 配对 pipe 两端连接（pipeId），供控制面关联 tunnel 两端的流视图；
	// 任一端无法提取连接编号时不配对，不影响数据通路
	p.state.LinkConns(networkOfURL(p.state.ListenURL), dialConn, networkOfURL(p.state.TargetURL), targetConn)

	start := time.Now()

	p.log.Info("pipe created", "from", dialer, "to", utils.RedactURL(p.state.TargetURL))
	dialRecv, dialSent, _ := utils.RelayConns(dialConn, targetConn)

	lifetimeInfo := fmt.Sprintf("sent=%v, recv=%v, lifetime=%v",
		humanize.IBytes(uint64(dialSent)), humanize.IBytes(uint64(dialRecv)), time.Since(start).Round(time.Second))

	p.log.Info("pipe stopped", "from", dialer, "to", utils.RedactURL(p.state.TargetURL), "stats", lifetimeInfo)
}
