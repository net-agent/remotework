package apiplugin

import (
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/vservice"
)

// tunnelSpecForTest 最小 TunnelSpec（fake catalog 不会执行插件 Init）
func tunnelSpecForTest() configv2.TunnelSpec {
	return configv2.TunnelSpec{ID: "t1", Name: "rdp"}
}

// fakeCatalog 可控的 Catalog 实现（只覆盖 waitSettled 用到的 State/Load/Unload）
type fakeCatalog struct {
	mu       sync.Mutex
	state    kernel.PluginState
	stateFn  func(call int) kernel.PluginState
	calls    int
	unloaded []string
	loads    int
}

func (c *fakeCatalog) List() []kernel.PluginState { return nil }

func (c *fakeCatalog) State(name string) (kernel.PluginState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.stateFn != nil {
		return c.stateFn(c.calls), nil
	}
	return c.state, nil
}

func (c *fakeCatalog) Subscribe(buf int) (<-chan kernel.Event, func()) {
	return make(chan kernel.Event), func() {}
}

func (c *fakeCatalog) Load(p kernel.Plugin) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loads++
	return nil
}

func (c *fakeCatalog) Unload(name string, opts kernel.UnloadOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unloaded = append(c.unloaded, name)
	return nil
}

func (c *fakeCatalog) Retry(name string) error { return nil }

// withWaitTimeout 缩短 waitSettled 超时并恢复
func withWaitTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	old := waitSettledTimeout
	waitSettledTimeout = d
	t.Cleanup(func() { waitSettledTimeout = old })
}

func TestWaitSettledRunning(t *testing.T) {
	fc := &fakeCatalog{state: kernel.PluginState{State: kernel.StateRunning}}
	b := &backend{catalog: fc}
	if err := b.waitSettled("tunnel/rdp"); err != nil {
		t.Fatalf("waitSettled running: %v", err)
	}
}

func TestWaitSettledPendingTimesOut(t *testing.T) {
	withWaitTimeout(t, 30*time.Millisecond)
	fc := &fakeCatalog{state: kernel.PluginState{
		State:   kernel.StatePending,
		Missing: []string{"network/link1", "vservice.Service"},
	}}
	b := &backend{catalog: fc}
	err := b.waitSettled("tunnel/rdp")
	if err == nil {
		t.Fatal("Pending must not be treated as success")
	}
	if !strings.Contains(err.Error(), "pending") || !strings.Contains(err.Error(), "network/link1") {
		t.Fatalf("error should mention pending/missing capabilities, got %v", err)
	}
	if len(fc.unloaded) != 0 {
		t.Fatalf("pending timeout should not unload, got %v", fc.unloaded)
	}
}

func TestWaitSettledPendingThenRunning(t *testing.T) {
	withWaitTimeout(t, time.Second)
	pending := kernel.PluginState{State: kernel.StatePending, Missing: []string{"network/link1"}}
	running := kernel.PluginState{State: kernel.StateRunning}
	fc := &fakeCatalog{stateFn: func(call int) kernel.PluginState {
		if call < 3 {
			return pending
		}
		return running
	}}
	b := &backend{catalog: fc}
	if err := b.waitSettled("tunnel/rdp"); err != nil {
		t.Fatalf("waitSettled should succeed after dependencies become ready: %v", err)
	}
}

func TestWaitSettledFailedUnloads(t *testing.T) {
	withWaitTimeout(t, time.Second)
	fc := &fakeCatalog{state: kernel.PluginState{
		State:   kernel.StateFailed,
		LastErr: "init exploded",
	}}
	b := &backend{catalog: fc}
	err := b.waitSettled("tunnel/bad")
	if err == nil || !strings.Contains(err.Error(), "init exploded") {
		t.Fatalf("want init error, got %v", err)
	}
	if len(fc.unloaded) != 1 || fc.unloaded[0] != "tunnel/bad" {
		t.Fatalf("failed plugin should be unloaded, got %v", fc.unloaded)
	}
}

func TestWaitSettledUnloaded(t *testing.T) {
	withWaitTimeout(t, time.Second)
	fc := &fakeCatalog{state: kernel.PluginState{State: kernel.StateUnloaded}}
	b := &backend{catalog: fc}
	if err := b.waitSettled("tunnel/gone"); err == nil {
		t.Fatal("unloaded plugin must return error")
	}
}

func TestMountTunnelWaitsSettled(t *testing.T) {
	withWaitTimeout(t, time.Second)
	fc := &fakeCatalog{state: kernel.PluginState{State: kernel.StateRunning}}
	b := &backend{catalog: fc}

	tunnelSpec := tunnelSpecForTest()
	if err := b.MountTunnel(tunnelSpec); err != nil {
		t.Fatalf("MountTunnel: %v", err)
	}
	if fc.loads != 1 {
		t.Fatalf("Load calls = %d, want 1", fc.loads)
	}
}

func TestMountTunnelFailedUnloads(t *testing.T) {
	withWaitTimeout(t, time.Second)
	fc := &fakeCatalog{state: kernel.PluginState{
		State:   kernel.StateFailed,
		LastErr: "boom",
	}}
	b := &backend{catalog: fc}
	err := b.MountTunnel(tunnelSpecForTest())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want boom error, got %v", err)
	}
	if len(fc.unloaded) != 1 {
		t.Fatalf("failed mount should unload plugin, got %v", fc.unloaded)
	}
}

// connIDConn 带 ConnID 的假连接，供 ServiceState.TrackConn/LinkConns 识别
type connIDConn struct {
	net.Conn
	id int64
}

func (c connIDConn) ConnID() int64 { return c.id }

// TestAnnotateWithServices 批量标注：首命中优先，归属与 pipe 一次算清
func TestAnnotateWithServices(t *testing.T) {
	svcA := &vservice.Service{}
	svcA.Name = "svc-a"
	svcA.TrackConn("mynet", connIDConn{id: 7})
	pipeID := svcA.LinkConns("mynet", connIDConn{id: 7}, "vtcp", connIDConn{id: 99})
	if pipeID == "" {
		t.Fatal("LinkConns should produce a pipe id")
	}

	// 同名连接被第二个服务登记时，不应覆盖先命中的归属
	svcB := &vservice.Service{}
	svcB.Name = "svc-b"
	svcB.TrackConn("mynet", connIDConn{id: 7})

	out := annotateWithServices([]*vservice.Service{svcA, svcB}, "mynet", []int64{7, 8})
	if len(out) != 2 {
		t.Fatalf("got %d annotations, want 2", len(out))
	}
	if out[7].ServiceName != "svc-a" {
		t.Errorf("annotation[7].ServiceName = %q, want svc-a (first match wins)", out[7].ServiceName)
	}
	if out[7].PipeID != pipeID {
		t.Errorf("annotation[7].PipeID = %q, want %q", out[7].PipeID, pipeID)
	}
	if out[8].ServiceName != "" || out[8].PipeID != "" {
		t.Errorf("annotation[8] should be empty, got %+v", out[8])
	}

	// nil 服务不应 panic
	if got := annotateWithServices([]*vservice.Service{nil}, "mynet", []int64{1}); len(got) != 1 {
		t.Fatalf("nil service annotation len = %d, want 1", len(got))
	}
}

// TestAnnotateWithServicesEmpty 空输入快速返回
func TestAnnotateWithServicesEmpty(t *testing.T) {
	if got := annotateWithServices(nil, "mynet", nil); got == nil {
		t.Fatal("nil service + nil indexes should still return non-nil map")
	}
}

// TestFormatPingDuration 既有格式化逻辑回归
func TestFormatPingDuration(t *testing.T) {
	cases := map[time.Duration]string{
		500 * time.Microsecond:  "<1ms",
		12 * time.Millisecond:   "12ms",
		1500 * time.Millisecond: "1.5s",
	}
	for dur, want := range cases {
		if got := formatPingDuration(dur); got != want {
			t.Errorf("formatPingDuration(%v) = %q, want %q", dur, got, want)
		}
	}
}
