package agent

import (
	"github.com/net-agent/remotework/agent/vnet"
	"github.com/net-agent/remotework/agent/vservice"
)

// 类型别名 — 保持向后兼容，外部消费者（api/、cmd/）无需修改

type QuickDialer = vservice.QuickDialer
type ListenerFactory = vservice.ListenerFactory
type DialerFactory = vservice.DialerFactory
type Network = vnet.Network
type NetworkMetaProvider = vnet.NetworkMetaProvider
type NetworkMeta = vnet.NetworkMeta
type NetworkReport = vnet.NetworkReport
type PingReport = vnet.PingReport
type ErrDependencyNotReady = vnet.ErrDependencyNotReady

var ErrPingNotSupported = vnet.ErrPingNotSupported
