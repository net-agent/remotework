package vservice

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/socks"
)

// socksHandshakeTimeout 限制客户端完成 SOCKS5 握手的时间，避免慢握手长期占用连接。
const socksHandshakeTimeout = 5 * time.Second

type Socks5Controller struct {
	state    *ServiceState
	log      *slog.Logger
	lf       ListenerFactory
	listener net.Listener
	server   socks.Server

	dialNetwork string       // 出站连接归属的网络别名（归属登记与拨号参数）
	dial        TargetDialer // 出站拨号能力；nil 时回退到 socks 库默认的 net.Dial

	// conns 已接受连接的登记表（含握手中的连接）；Close 时强杀
	conns activeConnRegistry
	// closeOnce/closeErr 保证 Close 幂等；Close 不等待 socks 库的 connWait
	closeOnce sync.Once
	closeErr  error
}

// NewSocks5Controller 创建 SOCKS5 控制器。
// log 为 nil 时回退到按服务名自建的模块日志器。
func NewSocks5Controller(lf ListenerFactory, state *ServiceState, log *slog.Logger, dialNetwork string, dial TargetDialer) *Socks5Controller {
	if log == nil {
		log = utils.NewModuleLogger("svc." + state.Name)
	}
	c := &Socks5Controller{
		state:       state,
		log:         log,
		lf:          lf,
		dialNetwork: dialNetwork,
		dial:        dial,
	}
	c.conns.max = defaultMaxActiveConnections
	return c
}

func (s *Socks5Controller) Init() error {
	s.server = socks.NewPswdServer(s.state.Username, s.state.Password)
	if s.dial != nil {
		// 出站拨号改经注入的网络能力发出，target 侧连接纳入流追踪
		s.server.SetRequster(s.requestTarget)
	}
	s.server.SetConnLinker(func(a, b io.ReadWriteCloser) (a2b int64, b2a int64, err error) {
		// 握手完成、进入转发阶段：清除握手读 deadline，并把连接标记为活跃
		if tc, ok := a.(interface{ markActive() }); ok {
			tc.markActive()
		}
		dialer := remoteInfoOf(a)
		start := time.Now()
		defer func() {
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

	raw, err := s.lf.ListenURL(s.state.ListenURL)
	if err != nil {
		return err
	}
	// 包装 listener：Accept 时设置握手 deadline、登记连接并计数
	s.listener = &socksTrackingListener{Listener: raw, ctl: s}
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
	err := s.server.Run(s.listener)
	// Close（含 tunnel Drain 直接关闭 listener）导致的退出是正常路径：
	// net.ErrClosed 与 controller 自身关闭都视为预期，不标记失败。
	if s.conns.isClosed() || errors.Is(err, net.ErrClosed) {
		if err != nil {
			s.log.Debug("socks5 run stopped", "err", err)
		}
		return err
	}
	if err == nil {
		err = errors.New("socks5 server stopped unexpectedly")
	}
	s.state.SetStatus(StatusFailed)
	s.state.SetLastErr(err)
	s.log.Error("socks5 run exited unexpectedly", "err", err)
	return err
}

// Close 关闭 listener 与全部已登记连接后立即返回。
// 不调用 socks.Server.Close：它会等待 connWait（阻塞），且在 Run 未启动时
// 会解引用 nil listener 而 panic。
func (s *Socks5Controller) Close() error {
	s.closeOnce.Do(func() {
		if s.listener != nil {
			if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				s.closeErr = err
			}
		}
		killed := s.conns.closeAll()
		if killed > 0 {
			s.log.Debug("closed active connections", "count", killed)
		}
	})
	return s.closeErr
}

// socksTrackingListener 在 Accept 时建立可追踪连接：
// 设置 5s 握手读 deadline、登记到控制器（Close 可强杀）并计入活跃数。
type socksTrackingListener struct {
	net.Listener
	ctl *Socks5Controller
}

func (l *socksTrackingListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		tc := &socksTrackedConn{
			Conn:       conn,
			ctl:        l.ctl,
			remoteInfo: GetRemoteInfo(conn), // 包装会遮蔽 *stream.Stream 断言，先缓存
		}
		tc.handshaking.Store(true)
		_ = conn.SetReadDeadline(time.Now().Add(socksHandshakeTimeout))

		// 先计数再登记：Close 并发强杀时由 Close 统一 AddDoneCount，
		// 登记失败则自行回滚，保证 actives/dones 不重复计数。
		l.ctl.state.AddActiveCount(1)
		if !l.ctl.conns.add(tc) {
			l.ctl.state.AddDoneCount(1)
			_ = conn.Close()
			if l.ctl.conns.isClosed() {
				return nil, net.ErrClosed
			}
			continue // 达到连接上限：丢弃新连接但保持服务
		}

		// 底层支持半关闭时返回半关闭型包装，避免遮蔽 RelayConns 依赖的接口
		rc, okR := conn.(closeReader)
		cw, okW := conn.(closeWriter)
		if okR && okW {
			return &socksTrackedHalfConn{socksTrackedConn: tc, rc: rc, cw: cw}, nil
		}
		return tc, nil
	}
}

// socksTrackedConn 是 listener 侧连接的登记包装：
// Close 幂等注销并计数；markActive 在握手结束、进入转发时清除 deadline。
type socksTrackedConn struct {
	net.Conn
	ctl         *Socks5Controller
	remoteInfo  string
	closeOnce   sync.Once
	handshaking atomic.Bool
}

func (c *socksTrackedConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.ctl.conns.remove(c)
		c.ctl.state.AddDoneCount(1)
		err = c.Conn.Close()
	})
	return err
}

// markActive 握手完成：清除握手读 deadline，标记连接已进入活跃转发阶段。
// 幂等；已关闭的连接不再设置 deadline。
func (c *socksTrackedConn) markActive() {
	if c.handshaking.CompareAndSwap(true, false) {
		_ = c.Conn.SetReadDeadline(time.Time{})
	}
}

// socksTrackedHalfConn 保留底层的 CloseRead/CloseWrite（RelayConns 依赖）
type socksTrackedHalfConn struct {
	*socksTrackedConn
	rc closeReader
	cw closeWriter
}

func (c *socksTrackedHalfConn) CloseRead() error  { return c.rc.CloseRead() }
func (c *socksTrackedHalfConn) CloseWrite() error { return c.cw.CloseWrite() }

// remoteInfoOf 读取连接的可读对端描述：包装连接优先取 Accept 时缓存的文本，
// 否则回退到 GetRemoteInfo 的类型断言。
func remoteInfoOf(c io.ReadWriteCloser) string {
	if ri, ok := c.(interface{ RemoteInfo() string }); ok {
		return ri.RemoteInfo()
	}
	return GetRemoteInfo(c)
}

// RemoteInfo 实现 remoteInfoOf 的缓存读取接口
func (c *socksTrackedConn) RemoteInfo() string { return c.remoteInfo }
