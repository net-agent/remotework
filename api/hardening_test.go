package api

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/utils"
)

// TestServerStopIdempotentAndBounded 回归 P1-7/P2：
// 退订通道不关闭时 Stop 也必须通过 stopCh 让 eventLoop 退出，
// 且重复调用 Stop 不 panic、不无限阻塞。
func TestServerStopIdempotentAndBounded(t *testing.T) {
	s := New(fakeBackend{}, Config{Listen: "127.0.0.1:0", Meta: testMeta}, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	done := make(chan struct{})
	go func() {
		s.Stop()
		s.Stop() // 幂等
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop blocked (event loop did not exit)")
	}

	select {
	case <-s.loopDone:
	default:
		t.Fatal("event loop should have exited")
	}
}

// TestPollerStopIdempotent Stop 可重复调用，且 Start 前调用也安全
func TestPollerStopIdempotent(t *testing.T) {
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))

	notStarted := NewStatePoller(fakeBackend{}, hub, nil, nil, time.Hour, time.Hour)
	notStarted.Stop() // Start 前调用：空操作

	poller := NewStatePoller(fakeBackend{}, hub, nil, nil, time.Hour, time.Hour)
	poller.Start()
	poller.Stop()
	poller.Stop()
}

// TestServerHTTPTimeouts HTTP 面必须设置读/写/空闲超时（P1-4）
func TestServerHTTPTimeouts(t *testing.T) {
	s := New(fakeBackend{}, Config{Listen: "127.0.0.1:0", Meta: testMeta}, nil)
	if s.server.ReadHeaderTimeout != apiReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v", s.server.ReadHeaderTimeout)
	}
	if s.server.ReadTimeout != apiReadTimeout {
		t.Errorf("ReadTimeout = %v", s.server.ReadTimeout)
	}
	if s.server.WriteTimeout != apiWriteTimeout {
		t.Errorf("WriteTimeout = %v", s.server.WriteTimeout)
	}
	if s.server.IdleTimeout != apiIdleTimeout {
		t.Errorf("IdleTimeout = %v", s.server.IdleTimeout)
	}
}

// tunnelBackend 记录动态挂载请求
type tunnelBackend struct {
	fakeBackend
	mounts []configv2.TunnelSpec
}

func (b *tunnelBackend) MountTunnel(spec configv2.TunnelSpec) error {
	b.mounts = append(b.mounts, spec)
	return nil
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// TestHandleAddTunnelValidation 回归 P1-8：空 id/name/listen/target 必须在挂载前被拒绝
func TestHandleAddTunnelValidation(t *testing.T) {
	b := &tunnelBackend{}
	s := startTestServerWith(t, b, Config{Meta: testMeta})
	url := "http://" + s.Addr() + "/api/v1/tunnels"

	cases := []string{
		`{}`,
		`{"id":"t1","name":"rdp","listen":"tcp://127.0.0.1:13389"}`,                    // 缺 target
		`{"id":"t1","name":"rdp","target":"tcp://127.0.0.1:3389"}`,                     // 缺 listen
		`{"id":"t1","listen":"tcp://127.0.0.1:13389","target":"tcp://127.0.0.1:3389"}`, // 缺 name
		`{"id":"  ","name":"rdp","listen":"tcp://127.0.0.1:13389","target":"tcp://127.0.0.1:3389"}`,
	}
	for _, body := range cases {
		resp := postJSON(t, url, body)
		env := decodeEnvelope(t, resp)
		if env.ErrCode == 0 {
			t.Fatalf("body %s should be rejected", body)
		}
	}
	if len(b.mounts) != 0 {
		t.Fatalf("invalid requests must not mount tunnels, got %d", len(b.mounts))
	}

	valid := `{"id":"t1","name":"rdp","listen":"tcp://127.0.0.1:13389","target":"tcp://127.0.0.1:3389"}`
	resp := postJSON(t, url, valid)
	env := decodeEnvelope(t, resp)
	if env.ErrCode != 0 {
		t.Fatalf("valid tunnel rejected: %s", env.ErrMsg)
	}
	if len(b.mounts) != 1 || b.mounts[0].Name != "rdp" {
		t.Fatalf("valid tunnel should be mounted once, got %+v", b.mounts)
	}
}

// TestReadJSONBodyLimit 超过 1MB 的请求体必须被拒绝（P1-4）
func TestReadJSONBodyLimit(t *testing.T) {
	b := &tunnelBackend{}
	s := startTestServerWith(t, b, Config{Meta: testMeta})
	url := "http://" + s.Addr() + "/api/v1/tunnels"

	big := `{"id":"t1","name":"` + strings.Repeat("a", 1<<20) + `"}`
	resp := postJSON(t, url, big)
	env := decodeEnvelope(t, resp)
	if env.ErrCode == 0 {
		t.Fatal("oversized body should be rejected")
	}
	if !strings.Contains(env.ErrMsg, "too large") {
		t.Errorf("want body-too-large error, got %q", env.ErrMsg)
	}
	if len(b.mounts) != 0 {
		t.Fatalf("oversized body must not mount tunnel, got %d", len(b.mounts))
	}
}

// TestListeningPortsCache /system/listening-ports 2s 内复用采集结果
func TestListeningPortsCache(t *testing.T) {
	b := &fakeBackend{}
	s := startTestServerWith(t, b, Config{Meta: testMeta})

	var calls atomic.Int32
	s.listPorts = func() ([]utils.ListeningPortInfo, error) {
		calls.Add(1)
		return []utils.ListeningPortInfo{{Port: 12345, Protocol: "tcp", Address: "127.0.0.1"}}, nil
	}
	url := "http://" + s.Addr() + "/api/v1/system/listening-ports"

	for i := 0; i < 3; i++ {
		resp := doRequest(t, "GET", url, "", nil)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), "12345") {
			t.Fatalf("response missing cached port: %s", body)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("within TTL: enumerate calls = %d, want 1", got)
	}

	// 手动让缓存过期 → 重新采集
	s.portsMu.Lock()
	s.portsAt = time.Now().Add(-listeningPortsCacheTTL - time.Second)
	s.portsMu.Unlock()
	resp := doRequest(t, "GET", url, "", nil)
	resp.Body.Close()
	if got := calls.Load(); got != 2 {
		t.Fatalf("after TTL: enumerate calls = %d, want 2", got)
	}
}

// slowPingBackend 模拟卡住的后端探测
type slowPingBackend struct {
	fakeBackend
	delay time.Duration
}

func (b *slowPingBackend) PingDomain(network, domain string) (time.Duration, error) {
	time.Sleep(b.delay)
	return time.Second, nil
}

// TestPingBusyIsRejected 并发上限：槽位耗尽时立即返回错误而不是排队
func TestPingBusyIsRejected(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	for i := 0; i < pingMaxConcurrent; i++ {
		s.pingSem <- struct{}{}
	}
	defer func() {
		for i := 0; i < pingMaxConcurrent; i++ {
			<-s.pingSem
		}
	}()

	resp := doRequest(t, "POST", "http://"+s.Addr()+"/api/v1/ping", "", nil)
	env := decodeEnvelope(t, resp)
	if env.ErrCode == 0 || !strings.Contains(env.ErrMsg, "too many") {
		t.Fatalf("want busy error, got code=%d msg=%q", env.ErrCode, env.ErrMsg)
	}
}

// TestPingSingleTimeout /ping 整请求超时：后端卡住时按上限返回错误
func TestPingSingleTimeout(t *testing.T) {
	old := pingSingleTimeout
	pingSingleTimeout = 50 * time.Millisecond
	defer func() { pingSingleTimeout = old }()

	b := &slowPingBackend{delay: 300 * time.Millisecond}
	s := startTestServerWith(t, b, Config{Meta: testMeta})

	start := time.Now()
	resp := doRequest(t, "POST", "http://"+s.Addr()+"/api/v1/ping/mynet/example.com", "", nil)
	elapsed := time.Since(start)
	env := decodeEnvelope(t, resp)
	if env.ErrCode == 0 || !strings.Contains(env.ErrMsg, "timeout") {
		t.Fatalf("want timeout error, got code=%d msg=%q", env.ErrCode, env.ErrMsg)
	}
	if elapsed > time.Second {
		t.Fatalf("ping timeout should return quickly, took %v", elapsed)
	}

	// 等后台探测自然结束，避免测试结束时 goroutine 仍在写
	time.Sleep(350 * time.Millisecond)
}

// TestPingAllSerializedBeyondLimit 批量 ping 的并发限制同样生效
func TestPingAllSerializedBeyondLimit(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	for i := 0; i < pingMaxConcurrent; i++ {
		s.pingSem <- struct{}{}
	}
	resp := doRequest(t, "POST", "http://"+s.Addr()+"/api/v1/ping", "", nil)
	env := decodeEnvelope(t, resp)
	if env.ErrCode == 0 {
		t.Fatal("batch ping should be rejected while slots are exhausted")
	}
	for i := 0; i < pingMaxConcurrent; i++ {
		<-s.pingSem
	}
}
