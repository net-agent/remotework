package kernel_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

// TestUnloadCleanupLIFOAndDone 卸载时：Done 先关闭，cleanup 严格 LIFO 执行
func TestUnloadCleanupLIFOAndDone(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	var ctxRef *kernel.Context
	mkCleanup := func(tag string) kernel.CleanupFunc {
		return func(ctx context.Context) error {
			// Done 必须在 cleanup 执行前已关闭
			select {
			case <-ctxRef.Done():
			default:
				t.Errorf("cleanup %s runs before Done closed", tag)
			}
			rec.add(tag)
			return nil
		}
	}

	p := &fakePlugin{
		info: kernel.Info{Name: "p"},
		initFn: func(c *kernel.Context) error {
			ctxRef = c
			c.DeferCleanup(mkCleanup("c1"))
			c.DeferCleanup(mkCleanup("c2"))
			return nil
		},
		startFn: func(c *kernel.Context) error {
			c.DeferCleanup(mkCleanup("c3"))
			return nil
		},
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateRunning)

	if err := h.Unload("p", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if !rec.before("c3", "c2") || !rec.before("c2", "c1") {
		t.Fatalf("cleanups must run in LIFO order, got %v", rec.ev)
	}
}

// TestUnloadGuardAndCascade 有依赖者时拒绝卸载；Cascade 时按逆依赖序级联卸载
func TestUnloadGuardAndCascade(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	// 依赖链：na <- nb <- nc
	na := &fakePlugin{
		info:   kernel.Info{Name: "na"},
		initFn: provideGreet("a"),
	}
	na.initFn = func(c *kernel.Context) error {
		c.DeferCleanup(func(ctx context.Context) error { rec.add("na.cleanup"); return nil })
		return provideGreet("a")(c)
	}
	nb := &fakePlugin{
		info: kernel.Info{Name: "nb"},
		initFn: func(c *kernel.Context) error {
			if _, err := kernel.Inject[greetCap](c, "a"); err != nil {
				return err
			}
			c.DeferCleanup(func(ctx context.Context) error { rec.add("nb.cleanup"); return nil })
			return kernel.Provide[greetCap](c, "b", greetImpl("b"))
		},
	}
	nc := &fakePlugin{
		info: kernel.Info{Name: "nc"},
		initFn: func(c *kernel.Context) error {
			if _, err := kernel.Inject[greetCap](c, "b"); err != nil {
				return err
			}
			c.DeferCleanup(func(ctx context.Context) error { rec.add("nc.cleanup"); return nil })
			return nil
		},
	}
	h.Load(na)
	h.Load(nb)
	h.Load(nc)
	waitState(t, h, "nc", kernel.StateRunning)

	// 非级联卸载被拒绝，并列出依赖者
	err := h.Unload("na", kernel.UnloadOptions{})
	var depErr *kernel.ErrHasDependents
	if !errors.As(err, &depErr) {
		t.Fatalf("expect ErrHasDependents, got %v", err)
	}
	if len(depErr.Dependents) != 2 {
		t.Fatalf("expect 2 transitive dependents, got %v", depErr.Dependents)
	}

	// 级联卸载：依赖者先清理，被依赖者最后
	if err := h.Unload("na", kernel.UnloadOptions{Cascade: true}); err != nil {
		t.Fatalf("cascade unload: %v", err)
	}
	if !rec.before("nc.cleanup", "nb.cleanup") || !rec.before("nb.cleanup", "na.cleanup") {
		t.Fatalf("cascade unload must follow reverse dependency order, got %v", rec.ev)
	}
	if len(h.List()) != 0 {
		t.Fatalf("all plugins should be unloaded, got %+v", h.List())
	}
}

// TestDrainBeforeCleanup 实现 Drainer 的插件在 cleanup 之前排水
func TestDrainBeforeCleanup(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	p := &drainFake{
		fakePlugin: &fakePlugin{
			info: kernel.Info{Name: "p"},
			initFn: func(c *kernel.Context) error {
				c.DeferCleanup(func(ctx context.Context) error { rec.add("cleanup"); return nil })
				return nil
			},
		},
		drainFn: func(deadline time.Duration) error {
			if deadline <= 0 {
				t.Error("drain deadline should be positive")
			}
			rec.add("drain")
			return nil
		},
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateRunning)

	if err := h.Unload("p", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if !rec.before("drain", "cleanup") {
		t.Fatalf("drain must run before cleanups, got %v", rec.ev)
	}
}

// TestShutdownOrder Shutdown 级联卸载全部插件且遵循逆依赖序；关闭后宿主不可用
func TestShutdownOrder(t *testing.T) {
	rec := &recorder{}
	h := kernel.NewHost(nil, nil)

	na := &fakePlugin{
		info: kernel.Info{Name: "na"},
		initFn: func(c *kernel.Context) error {
			c.DeferCleanup(func(ctx context.Context) error { rec.add("na.cleanup"); return nil })
			return provideGreet("a")(c)
		},
	}
	nb := &fakePlugin{
		info: kernel.Info{Name: "nb"},
		initFn: func(c *kernel.Context) error {
			if _, err := kernel.Inject[greetCap](c, "a"); err != nil {
				return err
			}
			c.DeferCleanup(func(ctx context.Context) error { rec.add("nb.cleanup"); return nil })
			return nil
		},
	}
	h.Load(na)
	h.Load(nb)

	// 等待两者 Running
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		list := h.List()
		if len(list) == 2 && list[0].State == kernel.StateRunning && list[1].State == kernel.StateRunning {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if !rec.before("nb.cleanup", "na.cleanup") {
		t.Fatalf("shutdown must unload in reverse dependency order, got %v", rec.ev)
	}
	if err := h.Load(&fakePlugin{info: kernel.Info{Name: "late"}}); err != kernel.ErrHostShutdown {
		t.Fatalf("load after shutdown should fail with ErrHostShutdown, got %v", err)
	}
	// Shutdown 幂等
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown should be nil, got %v", err)
	}
}

func TestUnloadNotFoundAndTwice(t *testing.T) {
	h := newTestHost(t, nil)

	if err := h.Unload("nope", kernel.UnloadOptions{}); err != kernel.ErrPluginNotFound {
		t.Fatalf("expect ErrPluginNotFound, got %v", err)
	}

	h.Load(&fakePlugin{info: kernel.Info{Name: "p"}})
	waitState(t, h, "p", kernel.StateRunning)
	if err := h.Unload("p", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if err := h.Unload("p", kernel.UnloadOptions{}); err != kernel.ErrPluginNotFound {
		t.Fatalf("second unload should be ErrPluginNotFound, got %v", err)
	}
}

// TestUnloadFailedPluginCleanup 失败插件卸载时，已提交的 cleanup 仍被执行（无泄漏）
func TestUnloadFailedPluginCleanup(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	p := &fakePlugin{
		info: kernel.Info{Name: "p"},
		initFn: func(c *kernel.Context) error {
			c.DeferCleanup(func(ctx context.Context) error { rec.add("cleanup"); return nil })
			return nil
		},
		startFn: func(c *kernel.Context) error {
			return errors.New("start boom")
		},
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateFailed)

	if err := h.Unload("p", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if rec.index("cleanup") < 0 {
		t.Fatal("committed cleanup must run on unload even for failed plugin")
	}
	if _, err := h.State("p"); err != kernel.ErrPluginNotFound {
		t.Fatalf("plugin should be removed, got %v", err)
	}
}

// TestCleanupErrorAggregation cleanup 错误汇聚返回但不阻断其余 cleanup
func TestCleanupErrorAggregation(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	p := &fakePlugin{
		info: kernel.Info{Name: "p"},
		initFn: func(c *kernel.Context) error {
			c.DeferCleanup(func(ctx context.Context) error { rec.add("ok"); return nil })
			c.DeferCleanup(func(ctx context.Context) error { return errors.New("cleanup boom") })
			return nil
		},
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateRunning)

	err := h.Unload("p", kernel.UnloadOptions{})
	if err == nil || !strings.Contains(err.Error(), "cleanup boom") {
		t.Fatalf("cleanup error should be aggregated, got %v", err)
	}
	if rec.index("ok") < 0 {
		t.Fatal("other cleanups must still run after a failing one")
	}
}
