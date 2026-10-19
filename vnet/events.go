package vnet

import "github.com/net-agent/flex/v3/stream"

// 流生命周期 EventBus 主题，payload 为 StreamEvent。
// tcp 网络（connTracker）与 flex 网络（node.Trace 钩子）都会发布；
// Closed 的时效略有差异：tcp 在连接完全关闭时即时发布，
// flex 在流完全关闭后的滞留期（约 2 秒）届满时发布。
const (
	TopicStreamOpen   = "stream.opened"
	TopicStreamClosed = "stream.closed"
)

// StreamEvent 流生命周期事件；State 为事件发生时刻的流状态快照
type StreamEvent struct {
	Network string
	State   *stream.State
}

// StreamTrace 流生命周期回调。回调在 tracker 锁外触发，必须非阻塞
// （kernel EventBus.Publish 满足该约束）。
type StreamTrace struct {
	OnStreamOpen   func(st *stream.State)
	OnStreamClosed func(st *stream.State)
}
