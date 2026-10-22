package vservice

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/utils"
)

// QuickDialer 快速创建连接的函数类型
type QuickDialer func() (net.Conn, error)

// TargetDialer 按网络与地址创建出站连接的能力（形状与 vnet.Network.Dial 一致），
// 用于 socks5 等目标地址在运行时才确定的服务
type TargetDialer func(network, addr string) (net.Conn, error)

// ListenerFactory 从 URL 创建 listener 的能力
type ListenerFactory interface {
	ListenURL(raw string) (net.Listener, error)
}

// DialerFactory 从 URL 创建 dialer 的能力
type DialerFactory interface {
	URLDialer(raw string) (QuickDialer, error)
}

// serviceIDSeq 进程内服务序号（ServiceState.ID 的来源）。
// 由构造函数分配，便于控制面在同一进程生命周期内稳定标识一个服务实例。
var serviceIDSeq atomic.Int32

func nextServiceID() int32 { return serviceIDSeq.Add(1) }

// ServiceStatus 服务状态枚举
type ServiceStatus int32

const (
	StatusUninit  ServiceStatus = iota
	StatusInit                  // 初始化中
	StatusRunning               // 运行中
	StatusStopped               // 已停止
	StatusFailed                // 初始化失败
	StatusPending               // 依赖未就绪，等待自动启动
)

func (s ServiceStatus) String() string {
	switch s {
	case StatusUninit:
		return "uninit"
	case StatusInit:
		return "init"
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	case StatusFailed:
		return "init failed"
	case StatusPending:
		return "pending"
	default:
		return "unknown"
	}
}

type Service struct {
	ServiceState
	Controller ServiceController
}

type ServiceController interface {
	Init() error
	Start() error
	Close() error
}

type ServiceState struct {
	TunnelID  string // tunnel UUID from config (v2)
	Type      string
	Name      string
	ListenURL string
	TargetURL string
	Username  string
	Password  string
	LastErr   string

	status  int32 // 使用 atomic 操作，存储 ServiceStatus
	ID      int32
	actives int32
	dones   int32

	// lastErrMu/lastErrText 保存运行时错误（如控制循环永久退出）。
	// 历史导出字段 LastErr 保持不变（避免无锁读取方引入数据竞争），
	// 运行时错误统一经 SetLastErr/GetLastErr 线程安全访问。
	lastErrMu   sync.Mutex
	lastErrText string

	// connRefs 连接登记表（key 为 connRef，零值可用）：
	// 记录服务经手的连接 {网络别名, 连接编号}，供 api 层反查连接归属
	connRefs sync.Map
	// pipeRefs 配对登记表（key 为 connRef，value 为 pipeID）：
	// 记录被 pipe 关联的两端连接，供 api 层把 tunnel 两端的流关联为端到端视图；
	// 由 LinkConns 写入，随 untrackConn 与 connRefs 同时注销
	pipeRefs sync.Map
	// pipeSeq 服务内 pipe 序号（pipeID 的组成部分，从 1 开始）
	pipeSeq atomic.Uint64
}

func (s *ServiceState) SetStatus(st ServiceStatus) { atomic.StoreInt32(&s.status, int32(st)) }
func (s *ServiceState) GetStatus() ServiceStatus   { return ServiceStatus(atomic.LoadInt32(&s.status)) }
func (s *ServiceState) StatusString() string       { return s.GetStatus().String() }

// SetLastErr 记录服务运行时错误（线程安全，空错误忽略）。
func (s *ServiceState) SetLastErr(err error) {
	if err == nil {
		return
	}
	s.lastErrMu.Lock()
	s.lastErrText = err.Error()
	s.lastErrMu.Unlock()
}

// GetLastErr 返回通过 SetLastErr 记录的运行时错误。
func (s *ServiceState) GetLastErr() string {
	s.lastErrMu.Lock()
	defer s.lastErrMu.Unlock()
	return s.lastErrText
}

func (s *ServiceState) AddActiveCount(n int32) { atomic.AddInt32(&s.actives, n) }
func (s *ServiceState) AddDoneCount(n int32) {
	atomic.AddInt32(&s.actives, -n)
	atomic.AddInt32(&s.dones, n)
}
func (s *ServiceState) GetActiveCount() int32        { return atomic.LoadInt32(&s.actives) }
func (s *ServiceState) GetDoneCount() int32          { return atomic.LoadInt32(&s.dones) }
func (s *ServiceState) IsListenDepend(n string) bool { return strings.HasPrefix(s.ListenURL, n) }
func (s *ServiceState) IsTargetDepend(n string) bool { return strings.HasPrefix(s.TargetURL, n) }
func (s *ServiceState) IsDepend(n string) bool {
	return s.IsListenDepend(n) || s.IsTargetDepend(n)
}

func GetRemoteInfo(c interface{}) string {
	if streamConn, ok := c.(*stream.Stream); ok {
		state := streamConn.GetState()
		return fmt.Sprintf("mnet://%v", state.Remote())
	}

	if netConn, ok := c.(net.Conn); ok {
		return fmt.Sprintf("tcp://%v", netConn.RemoteAddr().String())
	}

	return "invalid_conn"
}

// NewPortproxyService 创建端口转发服务。log 为 nil 时控制器自建日志器。
func NewPortproxyService(lf ListenerFactory, df DialerFactory, name, listenURL, targetURL string, log *slog.Logger) *Service {
	svc := &Service{}
	svc.Type = "portproxy"
	svc.ID = nextServiceID()
	svc.Name = utils.FirstString(name, "portproxy")
	svc.ListenURL = listenURL
	svc.TargetURL = targetURL
	svc.Controller = NewPortproxyController(lf, df, &svc.ServiceState, log)
	return svc
}

// NewSocks5Service 创建 SOCKS5 代理服务。
// dialNetwork/dial 指定出站连接使用的网络能力与归属别名（如 "tcp"），
// 出站连接由此纳入 network plugin 的流追踪；dial 为 nil 时回退到
// socks 库默认的 net.Dial（target 侧不产生可追踪连接）。
// log 为 nil 时控制器自建日志器。
func NewSocks5Service(lf ListenerFactory, name, listenURL, username, password, dialNetwork string, dial TargetDialer, log *slog.Logger) *Service {
	svc := &Service{}
	svc.Type = "socks5"
	svc.ID = nextServiceID()
	svc.Name = utils.FirstString(name, "socks5")
	svc.ListenURL = listenURL
	svc.Username = username
	svc.Password = password
	svc.Controller = NewSocks5Controller(lf, &svc.ServiceState, log, dialNetwork, dial)
	return svc
}

// activeConnRegistry 记录控制器已接受、尚未结束的连接，供 Close 时强杀。
// 零值可用。
//
// add 与 closeAll 通过 mu 串行化，保证 closeAll 之后不会再有连接被漏登记：
// closeAll 置 closed 后，add 一律拒绝（调用方负责关闭被拒绝的连接）。
// conns 使用 sync.Map 存放连接（key 为 net.Conn）。
type activeConnRegistry struct {
	mu     sync.Mutex
	closed bool
	max    int // 0 表示不限制
	count  int
	conns  sync.Map
}

// defaultMaxActiveConnections 是单个服务允许同时经手的连接上限。
// 防止公网/虚拟网络监听被连接洪泛耗尽 goroutine/fd。
const defaultMaxActiveConnections = 4096

// add 登记一条连接；控制器已进入关闭流程时返回 false。
func (r *activeConnRegistry) add(conn net.Conn) bool {
	if conn == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	if r.max > 0 && r.count >= r.max {
		return false
	}
	r.conns.Store(conn, struct{}{})
	r.count++
	return true
}

// remove 注销一条连接（幂等）。
func (r *activeConnRegistry) remove(conn net.Conn) {
	if conn == nil {
		return
	}
	if _, loaded := r.conns.LoadAndDelete(conn); loaded {
		r.mu.Lock()
		if r.count > 0 {
			r.count--
		}
		r.mu.Unlock()
	}
}

// isClosed 返回控制器是否已进入关闭流程。
func (r *activeConnRegistry) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// closeAll 标记关闭并关闭全部已登记连接，返回强杀的连接数。
// 第二次调用不再重复关闭。
func (r *activeConnRegistry) closeAll() int {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return 0
	}
	r.closed = true
	r.mu.Unlock()

	count := 0
	r.conns.Range(func(key, _ any) bool {
		if conn, ok := key.(net.Conn); ok {
			_ = conn.Close()
		}
		r.conns.Delete(key)
		count++
		return true
	})
	r.mu.Lock()
	r.count = 0
	r.mu.Unlock()
	return count
}
