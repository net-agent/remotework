package vnet

import (
	"errors"
	"fmt"
	"net"
	"time"
)

var ErrPingNotSupported = errors.New("ping not supported")

// Network 纯网络能力接口
type Network interface {
	GetName() string
	Dial(network, addr string) (net.Conn, error)
	Listen(network, addr string) (net.Listener, error)
	Ping(domain string, timeout time.Duration) (time.Duration, error)
	Stop()
}

// NetworkMetaProvider 可选接口，提供实现特有的元数据
type NetworkMetaProvider interface {
	Meta() NetworkMeta
}

// StreamCloser 可选接口：按流 State.Index 主动关闭一条连接（手动关闭）
type StreamCloser interface {
	CloseStream(index int32) bool
}

type NetworkMeta struct {
	Protocol   string
	Address    string
	Domain     string
	State      string
	LastErr    string
	Reconnects int64  // 累计断线重连次数（虚拟网络有效）
	Kind       string // "local" or "virtual"

	BytesRead    int64         // 节点累计读取字节数（虚拟网络有效）
	BytesWritten int64         // 节点累计写入字节数（虚拟网络有效）
	RTT          time.Duration // 最近一次心跳 RTT（虚拟网络有效，未测量为 0）
}

type NetworkReport struct {
	Name       string
	Protocol   string
	Address    string
	Domain     string
	Alive      time.Duration
	StartedAt  time.Time // 网络对象创建时刻（Alive 的绝对时间锚点）
	Listens    int32
	Accepts    int32
	Dials      int32
	State      string
	LastErr    string
	Reconnects int64
	Kind       string // "local" or "virtual"

	BytesRead    int64         // 节点累计读取字节数
	BytesWritten int64         // 节点累计写入字节数
	RTT          time.Duration // 最近一次心跳 RTT（0 表示未测量或不支持）
}

type PingReport struct {
	Network      string
	Domain       string
	PingResult   string
	UsedServices []string
}

// ErrDependencyNotReady 表示服务依赖的网络尚未注册
type ErrDependencyNotReady struct {
	Network string
}

func (e *ErrDependencyNotReady) Error() string {
	return fmt.Sprintf("dependency network '%s' not ready", e.Network)
}
