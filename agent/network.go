package agent

import (
	"net"
	"time"
)

type QuickDialer func() (net.Conn, error)
type Network interface {
	GetName() string
	Dial(network, addr string) (net.Conn, error)
	Listen(network, addr string) (net.Listener, error)
	Ping(domain string, timeout time.Duration) (time.Duration, error)
	Report() NetworkReport
	Stop()
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
	Sends    int64
	Recvs    int64
	State    string
	LastErr  string
}

type PingReport struct {
	Network      string
	Domain       string
	PingResult   string
	UsedServices []string
}
