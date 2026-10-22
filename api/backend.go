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
	GetAllServiceState() ([]*vservice.ServiceState, error)
	GetDataStreamState(limit int, networks ...string) []*DataStreamState
	GetPingState() ([]*vnet.PingReport, error)
	PingDomain(network, domain string) (time.Duration, error)

	// 流管理
	// CloseStream 手动关闭指定网络中的一条流（按 stream.State.Index）
	CloseStream(network string, index int32) error
	// GetStreamService 返回流的归属服务名，未识别返回 ""
	GetStreamService(network string, index int64) string
	// GetStreamPipe 返回流的 pipe 配对标识（tunnel 两端流携带相同值），未配对返回 ""
	GetStreamPipe(network string, index int64) string

	// 运行时挂载
	MountLink(alias string, link configv2.LinkSpec) error
	MountTunnel(tunnel configv2.TunnelSpec) error

	// 请求停止整个宿主进程
	RequestStop()

	// 事件订阅（每订阅者独立 channel，退订函数收尾）
	SubscribeNetworkState(buf int) (<-chan NetworkStateEvent, func())
	SubscribeServiceStatus(buf int) (<-chan ServiceStatusEvent, func())
	// SubscribeStreamEvents 订阅流生命周期事件（打开/关闭，事件驱动即时送达；
	// Closed 受 flex 滞留期影响约滞后 2 秒）。底层不支持的实现可返回已关闭的 channel。
	SubscribeStreamEvents(buf int) (<-chan StreamLifecycleEvent, func())
}

// StreamAnnotation 单条流的归属标注：所属服务名与 pipe 配对标识
type StreamAnnotation struct {
	ServiceName string
	PipeID      string
}

// StreamAnnotator 可选后端能力：一次批量查询多条流的归属标注。
// 实现方应在一次调用内只做一次服务枚举（如 catalog.List()），
// 避免逐流调用 GetStreamService/GetStreamPipe 造成的 O(流×隧道) 调度往返。
// 未实现该接口的 Backend 由 annotateStreams 回退到逐流查询。
type StreamAnnotator interface {
	AnnotateStreams(network string, indexes []int64) map[int64]StreamAnnotation
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

// StreamLifecycleEvent 流生命周期事件（事件驱动，非轮询差分）
type StreamLifecycleEvent struct {
	Network string
	Closed  bool          // false=打开，true=完全关闭
	State   *stream.State // 事件发生时刻的流状态快照
}
