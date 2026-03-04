package vservice

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/utils"
)

// QuickDialer 快速创建连接的函数类型
type QuickDialer func() (net.Conn, error)

// ListenerFactory 从 URL 创建 listener 的能力
type ListenerFactory interface {
	ListenURL(raw string) (net.Listener, error)
}

// DialerFactory 从 URL 创建 dialer 的能力
type DialerFactory interface {
	URLDialer(raw string) (QuickDialer, error)
}

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
}

func (s *ServiceState) SetStatus(st ServiceStatus) { atomic.StoreInt32(&s.status, int32(st)) }
func (s *ServiceState) GetStatus() ServiceStatus   { return ServiceStatus(atomic.LoadInt32(&s.status)) }
func (s *ServiceState) StatusString() string       { return s.GetStatus().String() }

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

// NewPortproxyService 创建端口转发服务
func NewPortproxyService(lf ListenerFactory, df DialerFactory, name, listenURL, targetURL string) *Service {
	svc := &Service{}
	svc.Type = "portproxy"
	svc.Name = utils.FirstString(name, "portproxy")
	svc.ListenURL = listenURL
	svc.TargetURL = targetURL
	svc.Controller = NewPortproxyController(lf, df, &svc.ServiceState)
	return svc
}

// NewSocks5Service 创建 SOCKS5 代理服务
func NewSocks5Service(lf ListenerFactory, name, listenURL, username, password string) *Service {
	svc := &Service{}
	svc.Type = "socks5"
	svc.Name = utils.FirstString(name, "socks5")
	svc.ListenURL = listenURL
	svc.Username = username
	svc.Password = password
	svc.Controller = NewSocks5Controller(lf, &svc.ServiceState)
	return svc
}
