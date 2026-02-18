package agent

// HubEventListener 可选的事件监听器，用于接收 Hub 内部状态变化通知
type HubEventListener interface {
	OnNetworkStateChange(name, oldState, newState string)
	OnServiceStatusChange(name string, oldStatus, newStatus ServiceStatus)
}

// NetworkStateNotifier 网络状态变化通知接口
// networkImpl 在状态变更时通过 type-assert notifier 来调用
type NetworkStateNotifier interface {
	NotifyNetworkStateChange(name, oldState, newState string)
}
