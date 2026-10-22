package vservice

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/net-agent/remotework/utils"
)

// accept 循环的可恢复错误退避参数
const (
	acceptRetryMin = 5 * time.Millisecond
	acceptRetryMax = time.Second
)

type PortproxyController struct {
	state    *ServiceState
	log      *slog.Logger
	lf       ListenerFactory
	df       DialerFactory
	listener net.Listener
	dialer   QuickDialer

	// conns 已接受连接的登记表；Close 时强杀全部活跃连接
	conns activeConnRegistry
	// closeOnce/closeErr 保证 Close 幂等（Drain 与卸载可能各触发一次）
	closeOnce sync.Once
	closeErr  error
}

// NewPortproxyController 创建端口转发控制器。
// log 为 nil 时回退到按服务名自建的模块日志器。
func NewPortproxyController(lf ListenerFactory, df DialerFactory, state *ServiceState, log *slog.Logger) *PortproxyController {
	if log == nil {
		log = utils.NewModuleLogger("svc." + state.Name)
	}
	p := &PortproxyController{
		state: state,
		log:   log,
		lf:    lf,
		df:    df,
	}
	p.conns.max = defaultMaxActiveConnections
	return p
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

	var backoff time.Duration
	for {
		conn, err := p.listener.Accept()
		if err == nil {
			backoff = 0
			// 先登记再启动 serve；Close 并发时登记失败返回；达到连接
			// 上限时丢弃新连接但保持 listener 继续服务。
			if !p.conns.add(conn) {
				_ = conn.Close()
				if p.conns.isClosed() {
					return net.ErrClosed
				}
				continue
			}
			go p.serve(conn)
			continue
		}

		// listener 已关闭：正常退出路径，不重试、不标记失败
		if errors.Is(err, net.ErrClosed) {
			return err
		}
		// 可恢复错误（临时错误、fd 耗尽等）指数退避后重试
		if isRetryableAcceptError(err) {
			backoff = nextAcceptBackoff(backoff)
			p.log.Warn("accept failed, retrying", "err", err, "backoff", backoff)
			time.Sleep(backoff)
			if p.conns.isClosed() {
				return net.ErrClosed
			}
			continue
		}

		p.failStart(err)
		return err
	}
}

func (p *PortproxyController) Close() error {
	p.closeOnce.Do(func() {
		// 先关闭登记表，避免 Accept 与 Close 竞态时漏杀新连接
		killed := p.conns.closeAll()
		if p.listener != nil {
			// Drain 可能已经关闭过 listener；重复关闭按正常处理，避免
			// cleanup 把 net.ErrClosed 当成失败上报给 Shutdown。
			if err := p.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				p.closeErr = err
			}
		}
		if killed > 0 {
			p.log.Debug("closed active connections", "count", killed)
		}
	})
	return p.closeErr
}

func (p *PortproxyController) serve(accepted net.Conn) {
	defer p.conns.remove(accepted)
	p.state.AddActiveCount(1)
	defer p.state.AddDoneCount(1)

	// 包装前先取对端信息：登记包装会遮蔽 GetRemoteInfo 的 *stream.Stream 断言
	dialer := GetRemoteInfo(accepted)
	// 登记 listen 侧连接归属（网络别名取自 ListenURL 的 scheme），供控制面反查；
	// 无法提取连接编号时原样返回，不影响数据通路
	dialConn := p.state.TrackConn(networkOfURL(p.state.ListenURL), accepted)
	defer dialConn.Close()

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

	p.log.Debug("pipe created", "from", dialer, "to", utils.RedactURL(p.state.TargetURL))
	dialRecv, dialSent, _ := utils.RelayConns(dialConn, targetConn)

	lifetimeInfo := fmt.Sprintf("sent=%v, recv=%v, lifetime=%v",
		humanize.IBytes(uint64(dialSent)), humanize.IBytes(uint64(dialRecv)), time.Since(start).Round(time.Second))

	p.log.Debug("pipe stopped", "from", dialer, "to", utils.RedactURL(p.state.TargetURL), "stats", lifetimeInfo)
}

// failStart 记录 accept 循环永久退出的状态与错误。
func (p *PortproxyController) failStart(err error) {
	if p.conns.isClosed() {
		return // Close 触发的退出不算失败
	}
	p.state.SetStatus(StatusFailed)
	p.state.SetLastErr(err)
	p.log.Error("accept loop exited unexpectedly", "err", err)
}

// isRetryableAcceptError 判断 Accept 错误是否值得退避重试：
// 临时错误（net.Error.Temporary）或 fd 耗尽（EMFILE/ENFILE）。
func isRetryableAcceptError(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Temporary() {
		return true
	}
	return errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE)
}

// nextAcceptBackoff 计算下一次退避时长（指数增长，上限 acceptRetryMax）。
func nextAcceptBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return acceptRetryMin
	}
	next := current * 2
	if next > acceptRetryMax {
		return acceptRetryMax
	}
	return next
}
