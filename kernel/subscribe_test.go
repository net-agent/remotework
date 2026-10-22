package kernel_test

import (
	"context"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

// waitEventChanClosed 等待订阅 channel 被关闭（先排空已投递事件）。
func waitEventChanClosed(t *testing.T, ch <-chan kernel.Event) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("subscriber channel was not closed")
		}
	}
}

// TestUnsubscribeClosesChannel 覆盖 P1-7：退订后 channel 必须关闭，
// 让 for range 的订阅适配器能够退出；重复退订必须幂等。
func TestUnsubscribeClosesChannel(t *testing.T) {
	h := newTestHost(t, nil)

	sub, unsub := h.Subscribe(8)
	unsub() // send 完成即被调度器接收，close 随后在同一调度回合内发生
	waitEventChanClosed(t, sub)

	unsub()
	unsub()
}

// TestShutdownClosesSubscribers Shutdown 关闭仍存活的订阅 channel，
// 且与单次 delete+close 的实现配合不会双关（双关会让调度器 panic）。
func TestShutdownClosesSubscribers(t *testing.T) {
	h := newTestHost(t, nil)

	sub, unsub := h.Subscribe(8)
	// 用一次 State 查询作为调度屏障，确保 cmdSubscribe 已被处理
	if _, err := h.State("missing"); err != kernel.ErrPluginNotFound {
		t.Fatalf("state barrier: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	waitEventChanClosed(t, sub)

	// 关闭后再退订：send 失败，退订函数必须是 no-op
	unsub()
}

// TestUnsubscribeBeforeShutdownNoDoubleClose 先退订再 Shutdown 不应双关 channel。
func TestUnsubscribeBeforeShutdownNoDoubleClose(t *testing.T) {
	h := newTestHost(t, nil)

	sub, unsub := h.Subscribe(8)
	if _, err := h.State("missing"); err != kernel.ErrPluginNotFound {
		t.Fatalf("state barrier: %v", err)
	}
	unsub()
	waitEventChanClosed(t, sub)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
