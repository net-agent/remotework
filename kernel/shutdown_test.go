package kernel_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

// TestShutdownWaitsForCleanup Shutdown 必须等 cleanup 栈真正执行完才返回，
// 不能在插件节点仍处于 Stopping 时提前返回。
func TestShutdownWaitsForCleanup(t *testing.T) {
	h := newTestHost(t, nil)

	cleanupDone := make(chan struct{})
	p := &fakePlugin{
		info: kernel.Info{Name: "p", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			return c.DeferCleanup(func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				close(cleanupDone)
				return nil
			})
		},
	}
	if err := h.Load(p); err != nil {
		t.Fatalf("load: %v", err)
	}
	waitState(t, h, "p", kernel.StateRunning)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	select {
	case <-cleanupDone:
	default:
		t.Fatal("Shutdown returned before cleanup finished")
	}
	if list := h.List(); len(list) != 0 {
		t.Fatalf("all plugins should be unloaded, got %+v", list)
	}
}

// TestShutdownAggregatesUnloadErrors Shutdown 返回所有插件 Drain/cleanup 错误的聚合，
// 供 cmd 据此判断关停是否完整（非零退出码）。
func TestShutdownAggregatesUnloadErrors(t *testing.T) {
	h := newTestHost(t, nil)

	cleanupBoom := &fakePlugin{
		info: kernel.Info{Name: "cleanup-boom", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			return c.DeferCleanup(func(ctx context.Context) error {
				return errors.New("cleanup boom")
			})
		},
	}
	drainBoom := &drainFake{
		fakePlugin: &fakePlugin{info: kernel.Info{Name: "drain-boom", Kind: "test"}},
		drainFn:    func(deadline time.Duration) error { return errors.New("drain boom") },
	}
	if err := h.Load(cleanupBoom); err != nil {
		t.Fatalf("load cleanup-boom: %v", err)
	}
	if err := h.Load(drainBoom); err != nil {
		t.Fatalf("load drain-boom: %v", err)
	}
	waitState(t, h, "cleanup-boom", kernel.StateRunning)
	waitState(t, h, "drain-boom", kernel.StateRunning)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := h.Shutdown(ctx)
	if err == nil {
		t.Fatal("Shutdown should return aggregated Drain/cleanup errors")
	}
	if !strings.Contains(err.Error(), "cleanup boom") {
		t.Fatalf("aggregated error should include cleanup error, got %v", err)
	}
	if !strings.Contains(err.Error(), "drain boom") {
		t.Fatalf("aggregated error should include drain error, got %v", err)
	}
}

// TestUnloadErrorNotIncludedInLaterShutdown 普通 Unload 的错误仍只由本次调用返回，
// 不应计入之后的 Shutdown 结果。
func TestUnloadErrorNotIncludedInLaterShutdown(t *testing.T) {
	h := newTestHost(t, nil)

	p := &fakePlugin{
		info: kernel.Info{Name: "p", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			return c.DeferCleanup(func(ctx context.Context) error {
				return errors.New("unload only boom")
			})
		},
	}
	if err := h.Load(p); err != nil {
		t.Fatalf("load: %v", err)
	}
	waitState(t, h, "p", kernel.StateRunning)

	if err := h.Unload("p", kernel.UnloadOptions{}); err == nil || !strings.Contains(err.Error(), "unload only boom") {
		t.Fatalf("ordinary Unload should return its cleanup error, got %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown should not include errors from earlier Unload, got %v", err)
	}
}
