package nettcp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/vnet"
)

// probePlugin 在 Init 中注入内置 tcp 网络并主动拨号，制造一条被 connTracker
// 追踪的活跃连接，供 cleanup 测试观察。
type probePlugin struct {
	info   kernel.Info
	initFn func(*kernel.Context) error
}

func (p *probePlugin) Info() kernel.Info { return p.info }
func (p *probePlugin) Init(c *kernel.Context) error {
	if p.initFn != nil {
		return p.initFn(c)
	}
	return nil
}
func (p *probePlugin) Start(*kernel.Context) error { return nil }

func waitPluginRunning(t *testing.T, h *kernel.Host, name string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		st, err := h.State(name)
		if err == nil && st.State == kernel.StateRunning {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("plugin %q did not reach running; last=%v err=%v", name, st.State, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestPlugin_CleanupStopsTrackedConns 验证插件注册的 DeferCleanup 会在卸载时
// 调用 network.Stop()，从而让 connTracker.CloseAll("stopped") 关闭活跃连接。
func TestPlugin_CleanupStopsTrackedConns(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen failed: %v", err)
	}
	defer echo.Close()
	serverDone := make(chan struct{})
	defer close(serverDone)
	go func() {
		conn, err := echo.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-serverDone
	}()

	h := kernel.NewHost(nil, nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.Shutdown(ctx)
	})

	if err := h.Load(New("tcp")); err != nil {
		t.Fatalf("load network plugin: %v", err)
	}
	waitPluginRunning(t, h, "network/tcp")

	var tracked net.Conn
	probe := &probePlugin{
		info: kernel.Info{Name: "probe", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			network, err := kernel.Inject[vnet.Network](c, "tcp")
			if err != nil {
				return err
			}
			tracked, err = network.Dial("tcp", echo.Addr().String())
			return err
		},
	}
	if err := h.Load(probe); err != nil {
		t.Fatalf("load probe plugin: %v", err)
	}
	waitPluginRunning(t, h, "probe")
	if tracked == nil {
		t.Fatal("probe should have dialed a tracked connection")
	}
	defer tracked.Close()

	// 连接应处于打开状态：短 deadline 读取超时
	_ = tracked.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 1)
	if _, err := tracked.Read(buf); err == nil {
		t.Fatal("tracked conn should be open before unload")
	} else if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("before unload read err = %v, want timeout", err)
	}

	if err := h.Unload("probe", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload probe: %v", err)
	}
	// 卸载网络插件触发 cleanup -> Stop -> CloseAll("stopped")
	if err := h.Unload("network/tcp", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload network plugin: %v", err)
	}

	_ = tracked.SetReadDeadline(time.Now().Add(time.Second))
	_, err = tracked.Read(buf)
	if err == nil {
		t.Fatal("tracked conn should be closed by network.Stop cleanup")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("tracked conn still open after cleanup, read timed out: %v", err)
	}
}
