package kernel_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

//
// 测试工具
//

// greetCap 测试用能力类型
type greetCap interface{ Greet() string }

type greetImpl string

func (g greetImpl) Greet() string { return string(g) }

// fakePlugin 可编程行为的测试插件
type fakePlugin struct {
	info    kernel.Info
	initFn  func(c *kernel.Context) error
	startFn func(c *kernel.Context) error
}

func (p *fakePlugin) Info() kernel.Info { return p.info }
func (p *fakePlugin) Init(c *kernel.Context) error {
	if p.initFn != nil {
		return p.initFn(c)
	}
	return nil
}
func (p *fakePlugin) Start(c *kernel.Context) error {
	if p.startFn != nil {
		return p.startFn(c)
	}
	return nil
}

// drainFake 额外实现 Drainer 的测试插件
type drainFake struct {
	*fakePlugin
	drainFn func(deadline time.Duration) error
}

func (d *drainFake) Drain(deadline time.Duration) error { return d.drainFn(deadline) }

// recorder 跨插件记录调用顺序
type recorder struct {
	mu sync.Mutex
	ev []string
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ev = append(r.ev, s)
}

func (r *recorder) index(s string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.ev {
		if e == s {
			return i
		}
	}
	return -1
}

// before 断言 a 与 b 均被记录且 a 在 b 之前
func (r *recorder) before(a, b string) bool {
	ia, ib := r.index(a), r.index(b)
	return ia >= 0 && ib >= 0 && ia < ib
}

func newTestHost(t *testing.T, opts *kernel.HostOptions) *kernel.Host {
	t.Helper()
	h := kernel.NewHost(nil, opts)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h.Shutdown(ctx)
	})
	return h
}

func waitState(t *testing.T, h *kernel.Host, name string, want kernel.State) kernel.PluginState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var st kernel.PluginState
	var err error
	for {
		st, err = h.State(name)
		if err == nil && st.State == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("plugin %q did not reach state %s; last=%s err=%v", name, want, st.State, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// provideGreet 返回一个在 Init 中发布 greetCap[name] 的 initFn
func provideGreet(name string) func(c *kernel.Context) error {
	return func(c *kernel.Context) error {
		return kernel.Provide[greetCap](c, name, greetImpl("hello-"+name))
	}
}

//
// 基础生命周期
//

func TestSinglePluginLifecycle(t *testing.T) {
	h := newTestHost(t, nil)
	rec := &recorder{}

	p := &fakePlugin{
		info:    kernel.Info{Name: "p1", Kind: "test"},
		initFn:  func(c *kernel.Context) error { rec.add("p1.init"); return nil },
		startFn: func(c *kernel.Context) error { rec.add("p1.start"); return nil },
	}
	if err := h.Load(p); err != nil {
		t.Fatalf("load: %v", err)
	}

	st := waitState(t, h, "p1", kernel.StateRunning)
	if st.Info.Name != "p1" || st.Info.Kind != "test" {
		t.Fatalf("unexpected info: %+v", st.Info)
	}
	if !rec.before("p1.init", "p1.start") {
		t.Fatalf("init should run before start, got %v", rec.ev)
	}

	if err := h.Unload("p1", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if _, err := h.State("p1"); err != kernel.ErrPluginNotFound {
		t.Fatalf("expect ErrPluginNotFound after unload, got %v", err)
	}
}

func TestLoadInvalidAndDuplicate(t *testing.T) {
	h := newTestHost(t, nil)

	if err := h.Load(nil); err != kernel.ErrInvalidPlugin {
		t.Fatalf("nil plugin: %v", err)
	}
	if err := h.Load(&fakePlugin{info: kernel.Info{Name: ""}}); err != kernel.ErrInvalidPlugin {
		t.Fatalf("empty name: %v", err)
	}

	p := &fakePlugin{info: kernel.Info{Name: "dup"}}
	if err := h.Load(p); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if err := h.Load(p); !strings.Contains(err.Error(), kernel.ErrPluginExists.Error()) {
		t.Fatalf("duplicate load: %v", err)
	}
	waitState(t, h, "dup", kernel.StateRunning)
}

func TestInjectOutsideCallbackFails(t *testing.T) {
	h := newTestHost(t, nil)

	var captured *kernel.Context
	p := &fakePlugin{
		info:   kernel.Info{Name: "p"},
		initFn: func(c *kernel.Context) error { captured = c; return nil },
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateRunning)

	if _, err := kernel.Inject[greetCap](captured, "x"); err == nil {
		t.Fatal("Inject outside callback should fail")
	}
	if err := kernel.Provide[greetCap](captured, "x", greetImpl("x")); err == nil {
		t.Fatal("Provide outside callback should fail")
	}
	if err := captured.DeferCleanup(func(ctx context.Context) error { return nil }); err == nil {
		t.Fatal("DeferCleanup outside callback should fail")
	}
}

func TestPanicIsolation(t *testing.T) {
	h := newTestHost(t, nil)

	boom := &fakePlugin{
		info:   kernel.Info{Name: "boom"},
		initFn: func(c *kernel.Context) error { panic("boom") },
	}
	h.Load(boom)
	st := waitState(t, h, "boom", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "panic") {
		t.Fatalf("expect panic error, got %q", st.LastErr)
	}

	// 宿主与其他插件不受影响
	ok := &fakePlugin{info: kernel.Info{Name: "ok"}, initFn: provideGreet("ok")}
	h.Load(ok)
	waitState(t, h, "ok", kernel.StateRunning)
}

func TestInitTimeout(t *testing.T) {
	h := newTestHost(t, &kernel.HostOptions{InitTimeout: 100 * time.Millisecond})

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	slow := &fakePlugin{
		info:   kernel.Info{Name: "slow"},
		initFn: func(c *kernel.Context) error { <-block; return nil },
	}
	h.Load(slow)
	st := waitState(t, h, "slow", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "timeout") {
		t.Fatalf("expect timeout error, got %q", st.LastErr)
	}

	// 宿主仍然响应
	ok := &fakePlugin{info: kernel.Info{Name: "ok"}}
	h.Load(ok)
	waitState(t, h, "ok", kernel.StateRunning)
}

func TestEventSubscription(t *testing.T) {
	h := newTestHost(t, nil)

	sub, unsub := h.Subscribe(32)
	defer unsub()

	h.Load(&fakePlugin{info: kernel.Info{Name: "p"}})
	waitState(t, h, "p", kernel.StateRunning)

	var evts []kernel.Event
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		select {
		case e := <-sub:
			evts = append(evts, e)
			if e.Kind == kernel.EventStateChanged && e.To == kernel.StateRunning {
				goto collected
			}
		case <-time.After(10 * time.Millisecond):
		}
	}
collected:
	if len(evts) == 0 {
		t.Fatal("no events received")
	}
	if evts[0].Kind != kernel.EventLoaded {
		t.Fatalf("first event should be loaded, got %v", evts[0].Kind)
	}
	last := evts[len(evts)-1]
	if last.Kind != kernel.EventStateChanged || last.To != kernel.StateRunning {
		t.Fatalf("last event should be running transition, got %+v", last)
	}
}

func TestStartFailureAndRetry(t *testing.T) {
	h := newTestHost(t, nil)

	var initCount, startCount atomic.Int32
	p := &fakePlugin{
		info: kernel.Info{Name: "p"},
		initFn: func(c *kernel.Context) error {
			initCount.Add(1)
			return nil
		},
		startFn: func(c *kernel.Context) error {
			if startCount.Add(1) == 1 {
				return context.DeadlineExceeded // 首次 Start 失败
			}
			return nil
		},
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateFailed)

	if err := h.Retry("p"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitState(t, h, "p", kernel.StateRunning)
	if initCount.Load() != 1 {
		t.Fatalf("Start 失败后 Retry 不应重新 Init, initCount=%d", initCount.Load())
	}
	if startCount.Load() != 2 {
		t.Fatalf("startCount=%d, want 2", startCount.Load())
	}
}

func TestInitFailureRetry(t *testing.T) {
	h := newTestHost(t, nil)

	var initCount atomic.Int32
	p := &fakePlugin{
		info: kernel.Info{Name: "p"},
		initFn: func(c *kernel.Context) error {
			if initCount.Add(1) == 1 {
				return context.DeadlineExceeded
			}
			return nil
		},
	}
	h.Load(p)
	waitState(t, h, "p", kernel.StateFailed)

	if err := h.Retry("p"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitState(t, h, "p", kernel.StateRunning)
	if initCount.Load() != 2 {
		t.Fatalf("initCount=%d, want 2", initCount.Load())
	}
}
