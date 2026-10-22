package vnet

import (
	"net"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

// tcpDialTimeout 是本地 TCP 拨号的建连超时
const tcpDialTimeout = 10 * time.Second

// tcpDialer 复用带超时的拨号器（net.Dialer 并发安全）
var tcpDialer = &net.Dialer{Timeout: tcpDialTimeout}

// tcpNetwork 包装标准 TCP 网络（本地网络），经 connTracker 追踪连接状态
type tcpNetwork struct {
	name    string
	tracker *connTracker
}

// NewTcpNetwork 创建本地 TCP 网络；trace 可为 nil（不发布流生命周期事件）
func NewTcpNetwork(name string, trace *StreamTrace) *tcpNetwork {
	return &tcpNetwork{name: name, tracker: newConnTracker(name, trace)}
}

func (tcp *tcpNetwork) GetName() string { return tcp.name }

func (tcp *tcpNetwork) Dial(network, addr string) (net.Conn, error) {
	conn, err := tcpDialer.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	return tcp.tracker.track(conn, stream.DirectionOutbound), nil
}

func (tcp *tcpNetwork) Listen(network, addr string) (net.Listener, error) {
	ln, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	return &trackedListener{Listener: ln, tracker: tcp.tracker}, nil
}

func (tcp *tcpNetwork) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	return 0, ErrPingNotSupported
}

// GetStreamStates 实现 registry 的 streamStateProvider 可选接口
func (tcp *tcpNetwork) GetStreamStates() (actives, closeds []*stream.State) {
	return tcp.tracker.GetStreamStates()
}

// CloseStream 实现 StreamCloser 可选接口：按 State.Index 手动关闭一条活跃连接
func (tcp *tcpNetwork) CloseStream(index int32) bool {
	return tcp.tracker.CloseStream(index)
}

func (tcp *tcpNetwork) Meta() NetworkMeta {
	// 本地网络不汇总节点级流量/RTT：BytesRead/BytesWritten/RTT 保持零值
	return NetworkMeta{
		Protocol: "-",
		Address:  "-",
		Domain:   "-",
		State:    "online",
		Kind:     "local",
	}
}

func (tcp *tcpNetwork) Stop() { tcp.tracker.CloseAll("stopped") }
