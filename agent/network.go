package agent

import (
	"errors"
	"net"
	"time"
)

var ErrPingNotSupported = errors.New("ping not supported")

type QuickDialer func() (net.Conn, error)

type Network interface {
	GetName() string
	Dial(network, addr string) (net.Conn, error)
	Listen(network, addr string) (net.Listener, error)
	Ping(domain string, timeout time.Duration) (time.Duration, error)
	Report() NetworkReport
	Stop()
}

// ListenerFactory 创建 listener 的能力（从 URL）
type ListenerFactory interface {
	ListenURL(raw string) (net.Listener, error)
}

// DialerFactory 创建 dialer 的能力（从 URL）
type DialerFactory interface {
	URLDialer(raw string) (QuickDialer, error)
}

// RawDialer 提供原始网络拨号能力
type RawDialer interface {
	Dial(network, addr string) (net.Conn, error)
}

// NetworkUpdateNotifier 网络重连后通知依赖方更新
type NetworkUpdateNotifier interface {
	UpdateNetwork(network string)
}

type NetworkReport struct {
	Name     string
	Protocol string
	Address  string
	Domain   string
	Alive    time.Duration
	Listens  int32
	Accepts  int32
	Dials    int32
	State    string
	LastErr  string
}

type PingReport struct {
	Network      string
	Domain       string
	PingResult   string
	UsedServices []string
}
