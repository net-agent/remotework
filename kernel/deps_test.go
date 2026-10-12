package kernel_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

// TestDependencyOrdering 依赖者先装载时进入 Pending；
// 提供者就绪后依赖者自动启动，且 Start 严格发生在提供者 Running 之后。
func TestDependencyOrdering(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	// nb 依赖 greetCap["a"]（由 na 提供）
	nb := &fakePlugin{
		info: kernel.Info{Name: "nb"},
		initFn: func(c *kernel.Context) error {
			g, err := kernel.Inject[greetCap](c, "a")
			if err != nil {
				return err
			}
			if g.Greet() != "hello-a" {
				t.Errorf("unexpected greet: %s", g.Greet())
			}
			return nil
		},
		startFn: func(c *kernel.Context) error { rec.add("nb.start"); return nil },
	}
	na := &fakePlugin{
		info:    kernel.Info{Name: "na"},
		initFn:  provideGreet("a"),
		startFn: func(c *kernel.Context) error { rec.add("na.start"); return nil },
	}

	// 先装载依赖者
	if err := h.Load(nb); err != nil {
		t.Fatalf("load nb: %v", err)
	}
	st := waitState(t, h, "nb", kernel.StatePending)
	if len(st.Missing) == 0 {
		t.Fatalf("pending plugin should report missing capabilities, got %+v", st)
	}

	// 再装载提供者：nb 应自动收敛到 Running
	h.Load(na)
	waitState(t, h, "na", kernel.StateRunning)
	waitState(t, h, "nb", kernel.StateRunning)

	if !rec.before("na.start", "nb.start") {
		t.Fatalf("provider must start before dependent, got %v", rec.ev)
	}
}

// TestOptionalInject 可选依赖不构成依赖图边，缺失不阻塞启动
func TestOptionalInject(t *testing.T) {
	h := newTestHost(t, nil)

	var gotAbsent, gotPresent atomic.Bool
	p := &fakePlugin{
		info: kernel.Info{Name: "p"},
		initFn: func(c *kernel.Context) error {
			if _, ok := kernel.InjectOptional[greetCap](c, "absent"); !ok {
				gotAbsent.Store(true)
			}
			return nil
		},
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateRunning)
	if !gotAbsent.Load() {
		t.Fatal("optional inject of absent capability should return ok=false")
	}

	h.Load(&fakePlugin{info: kernel.Info{Name: "q"}, initFn: provideGreet("present")})
	waitState(t, h, "q", kernel.StateRunning)

	r := &fakePlugin{
		info: kernel.Info{Name: "r"},
		initFn: func(c *kernel.Context) error {
			if _, ok := kernel.InjectOptional[greetCap](c, "present"); ok {
				gotPresent.Store(true)
			}
			return nil
		},
	}
	h.Load(r)
	waitState(t, h, "r", kernel.StateRunning)
	if !gotPresent.Load() {
		t.Fatal("optional inject of present capability should return ok=true")
	}
}

// TestInitFailureRollback Init 失败：暂存能力被丢弃、已登记 cleanup 自动回滚；
// 修复后 Retry 成功，因能力不可见而 Pending 的依赖者随后自动启动。
func TestInitFailureRollback(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	var attempts atomic.Int32
	p1 := &fakePlugin{
		info: kernel.Info{Name: "p1"},
		initFn: func(c *kernel.Context) error {
			if err := kernel.Provide[greetCap](c, "x", greetImpl("x")); err != nil {
				return err
			}
			c.DeferCleanup(func(ctx context.Context) error {
				rec.add("p1.rollback")
				return nil
			})
			if attempts.Add(1) == 1 {
				return errors.New("boom")
			}
			return nil
		},
	}
	h.Load(p1)
	waitState(t, h, "p1", kernel.StateFailed)

	// 失败 attempt 的 cleanup 已回滚执行
	if rec.index("p1.rollback") < 0 {
		t.Fatal("staged cleanup should be rolled back on init failure")
	}

	// p1 暂存的能力不可见：依赖者保持 Pending 而非启动
	p2 := &fakePlugin{
		info: kernel.Info{Name: "p2"},
		initFn: func(c *kernel.Context) error {
			_, err := kernel.Inject[greetCap](c, "x")
			return err
		},
	}
	h.Load(p2)
	waitState(t, h, "p2", kernel.StatePending)
	time.Sleep(100 * time.Millisecond)
	if st, _ := h.State("p2"); st.State != kernel.StatePending {
		t.Fatalf("p2 should stay pending while capability invisible, got %s", st.State)
	}

	// 修复 p1 后 Retry：p1 Running，p2 自动收敛 Running
	if err := h.Retry("p1"); err != nil {
		t.Fatalf("retry p1: %v", err)
	}
	waitState(t, h, "p1", kernel.StateRunning)
	waitState(t, h, "p2", kernel.StateRunning)
}

// TestCycleDetection 互相等待对方发布能力的插件构成环，全部判 Failed
func TestCycleDetection(t *testing.T) {
	h := newTestHost(t, nil)

	// ca：发布 x，依赖 y；cb：发布 y，依赖 x
	ca := &fakePlugin{
		info: kernel.Info{Name: "ca"},
		initFn: func(c *kernel.Context) error {
			if err := kernel.Provide[greetCap](c, "x", greetImpl("x")); err != nil {
				return err
			}
			_, err := kernel.Inject[greetCap](c, "y")
			return err
		},
	}
	cb := &fakePlugin{
		info: kernel.Info{Name: "cb"},
		initFn: func(c *kernel.Context) error {
			if err := kernel.Provide[greetCap](c, "y", greetImpl("y")); err != nil {
				return err
			}
			_, err := kernel.Inject[greetCap](c, "x")
			return err
		},
	}

	h.Load(ca)
	waitState(t, h, "ca", kernel.StatePending)
	h.Load(cb)

	for _, name := range []string{"ca", "cb"} {
		st := waitState(t, h, name, kernel.StateFailed)
		if !strings.Contains(st.LastErr, "cycle") {
			t.Fatalf("plugin %q should fail with cycle error, got %q", name, st.LastErr)
		}
	}
}

// TestDuplicateProvide 能力键冲突：后发布者 Failed，先发布者不受影响
func TestDuplicateProvide(t *testing.T) {
	h := newTestHost(t, nil)

	h.Load(&fakePlugin{info: kernel.Info{Name: "first"}, initFn: provideGreet("dup")})
	waitState(t, h, "first", kernel.StateRunning)

	second := &fakePlugin{
		info: kernel.Info{Name: "second"},
		initFn: func(c *kernel.Context) error {
			return kernel.Provide[greetCap](c, "dup", greetImpl("dup2"))
		},
	}
	h.Load(second)
	st := waitState(t, h, "second", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "already provided") {
		t.Fatalf("expect duplicate capability error, got %q", st.LastErr)
	}
	if st, _ := h.State("first"); st.State != kernel.StateRunning {
		t.Fatalf("first should stay running, got %s", st.State)
	}
}

// TestInjectSelfRejected 插件不能注入自己发布的能力（防止自依赖死锁）
func TestInjectSelfRejected(t *testing.T) {
	h := newTestHost(t, nil)

	p := &fakePlugin{
		info:   kernel.Info{Name: "selfish"},
		initFn: provideGreet("s"),
		startFn: func(c *kernel.Context) error {
			_, err := kernel.Inject[greetCap](c, "s") // Init 已提交，Start 时命中自注入守卫
			return err
		},
	}
	h.Load(p)
	st := waitState(t, h, "selfish", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "its own capability") {
		t.Fatalf("expect self-injection error, got %q", st.LastErr)
	}
}
