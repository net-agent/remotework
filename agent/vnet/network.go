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

type NetworkMeta struct {
	Protocol string
	Address  string
	Domain   string
	State    string
	LastErr  string
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

// ErrDependencyNotReady 表示服务依赖的网络尚未注册
type ErrDependencyNotReady struct {
	Network string
}

func (e *ErrDependencyNotReady) Error() string {
	return fmt.Sprintf("dependency network '%s' not ready", e.Network)
}
