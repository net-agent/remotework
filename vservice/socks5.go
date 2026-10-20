package vservice

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
	state    *ServiceState
	log      *slog.Logger
	lf       ListenerFactory
	listener net.Listener
	server   socks.Server

	dialNetwork string       // 出站连接归属的网络别名（归属登记与拨号参数）
	dial        TargetDialer // 出站拨号能力；nil 时回退到 socks 库默认的 net.Dial
}

// NewSocks5Controller 创建 SOCKS5 控制器。
// log 为 nil 时回退到按服务名自建的模块日志器。
func NewSocks5Controller(lf ListenerFactory, state *ServiceState, log *slog.Logger, dialNetwork string, dial TargetDialer) *Socks5Controller {
	if log == nil {
		log = utils.NewModuleLogger("svc." + state.Name)
	}
	return &Socks5Controller{
		state:       state,
		log:         log,
		lf:          lf,
		dialNetwork: dialNetwork,
		dial:        dial,
	}
}

func (s *Socks5Controller) Init() error {
	s.server = socks.NewPswdServer(s.state.Username, s.state.Password)
	if s.dial != nil {
		// 出站拨号改经注入的网络能力发出，target 侧连接纳入流追踪
		s.server.SetRequster(s.requestTarget)
	}
	s.server.SetConnLinker(func(a, b io.ReadWriteCloser) (a2b int64, b2a int64, err error) {
		dialer := GetRemoteInfo(a)
		start := time.Now()
		s.state.AddActiveCount(1)
		defer func() {
			s.state.AddDoneCount(1)
			a.Close()
			b.Close()
			s.log.Info("link stopped", "from", dialer, "alive", time.Since(start).Round(time.Second))
		}()
		// a 是 listen 侧接受的连接（运行时为 net.Conn），登记归属供控制面反查；
		// 无法提取连接编号时原样返回，不影响数据通路
		var aConn, bConn net.Conn
		if conn, ok := a.(net.Conn); ok {
			aConn = s.state.TrackConn(networkOfURL(s.state.ListenURL), conn)
			a = aConn
		}
		// b 是 target 侧出站连接：requestTarget 经 network plugin 拨号产生，
		// 携带连接编号（tcp ConnID），登记归属；库默认拨号（dial 为 nil）时
		// 无法提取编号，TrackConn 原样返回
		if conn, ok := b.(net.Conn); ok {
			bConn = s.state.TrackConn(s.dialNetwork, conn)
			b = bConn
		}
		// 配对 pipe 两端连接（pipeId），供控制面关联两端流视图
		if aConn != nil && bConn != nil {
			s.state.LinkConns(networkOfURL(s.state.ListenURL), aConn, s.dialNetwork, bConn)
		}
		s.log.Info("link created", "from", dialer)
		return utils.RelayConns(a, b)
	})

	var err error
	s.listener, err = s.lf.ListenURL(s.state.ListenURL)
	if err != nil {
		return err
	}
	return nil
}

// requestTarget 替换 socks 库的 DefaultRequester：出站拨号改经注入的网络能力发出。
// 命令分发语义与库默认实现保持一致（仅支持 CONNECT）。
func (s *Socks5Controller) requestTarget(req socks.Request, _ socks.Context) (net.Conn, error) {
	if req.GetCommand() != socks.ConnectCommand {
		return nil, socks.ErrReplyCmdNotSupported
	}
	return s.dial(s.dialNetwork, req.GetAddrPortStr())
}

func (s *Socks5Controller) Start() error {
	if s.server == nil || s.listener == nil {
		return errors.New("init failed")
	}
	return s.server.Run(s.listener)
}

func (s *Socks5Controller) Close() error {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.server != nil {
		s.server.Close()
	}
	return nil
}
