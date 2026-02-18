package agent

import (
	"net"
	"sync"
)

// HotSwapListener 提供可热替换的 listener，支持在网络重连后无缝切换。
// 被 PortproxyController 和 Socks5Controller 共同使用。
type HotSwapListener struct {
	mu       sync.Mutex
	listener net.Listener
	listenFn func() (net.Listener, error)
}

func NewHotSwapListener(listenFn func() (net.Listener, error)) *HotSwapListener {
	return &HotSwapListener{listenFn: listenFn}
}

// Refresh 创建新的 listener 并替换旧的
func (h *HotSwapListener) Refresh() error {
	l, err := h.listenFn()
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil {
		h.listener.Close()
	}
	h.listener = l
	return nil
}

// Get 获取当前 listener
func (h *HotSwapListener) Get() net.Listener {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.listener
}

// Close 关闭当前 listener 并置 nil，幂等安全
func (h *HotSwapListener) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil {
		err := h.listener.Close()
		h.listener = nil
		return err
	}
	return nil
}
