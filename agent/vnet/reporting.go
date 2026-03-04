package vnet

import (
	"net"
	"sync/atomic"
	"time"
)

// ReportingNetwork 装饰器，包装 Network 统一追踪通用指标
type ReportingNetwork struct {
	Network
	start       time.Time
	dialCount   atomic.Int32
	listenCount atomic.Int32
}

func NewReportingNetwork(n Network) *ReportingNetwork {
	return &ReportingNetwork{Network: n, start: time.Now()}
}

func (r *ReportingNetwork) Dial(network, addr string) (net.Conn, error) {
	r.dialCount.Add(1)
	return r.Network.Dial(network, addr)
}

func (r *ReportingNetwork) Listen(network, addr string) (net.Listener, error) {
	r.listenCount.Add(1)
	return r.Network.Listen(network, addr)
}

func (r *ReportingNetwork) Report() NetworkReport {
	report := NetworkReport{
		Name:    r.GetName(),
		Alive:   time.Since(r.start),
		Dials:   r.dialCount.Load(),
		Listens: r.listenCount.Load(),
	}
	if mp, ok := r.Network.(NetworkMetaProvider); ok {
		meta := mp.Meta()
		report.Protocol = meta.Protocol
		report.Address = meta.Address
		report.Domain = meta.Domain
		report.State = meta.State
		report.LastErr = meta.LastErr
	}
	return report
}
