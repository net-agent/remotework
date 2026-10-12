package api

import (
	"time"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// Backend 控制面后端抽象：api 包不绑定具体运行时，
// 由宿主侧（plugins/api）实现并注入。
type Backend interface {
	// 状态查询
	IsRunning() bool
	GetAllNetworkState() ([]vnet.NetworkReport, error)
	GetAllServiceState() ([]vservice.ServiceState, error)
	GetDataStreamState(limit int, networks ...string) []*DataStreamState
	GetPingState() ([]*vnet.PingReport, error)
	PingDomain(network, domain string) (time.Duration, error)

	// 运行时挂载
	MountLink(alias string, link configv2.LinkSpec) error
	MountTunnel(tunnel configv2.TunnelSpec) error

	// 请求停止整个宿主进程
	RequestStop()

	// 事件订阅（每订阅者独立 channel，退订函数收尾）
	SubscribeNetworkState(buf int) (<-chan NetworkStateEvent, func())
	SubscribeServiceStatus(buf int) (<-chan ServiceStatusEvent, func())
}

// DataStreamState 单个网络的数据流快照
type DataStreamState struct {
	Network string
	Actives []*stream.State
	Closeds []*stream.State
}

// NetworkStateEvent 网络会话状态变化事件
type NetworkStateEvent struct {
	Network  string
	OldState string
	NewState string
}

// ServiceStatusEvent 服务状态变化事件
type ServiceStatusEvent struct {
	Name      string
	OldStatus string
	NewStatus string
}
