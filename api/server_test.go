package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// fakeBackend 空实现的控制面后端，仅用于服务器层测试
type fakeBackend struct{}

func (fakeBackend) IsRunning() bool                                   { return true }
func (fakeBackend) GetAllNetworkState() ([]vnet.NetworkReport, error) { return nil, nil }
func (fakeBackend) GetAllServiceState() ([]vservice.ServiceState, error) {
	return nil, nil
}
func (fakeBackend) GetDataStreamState(limit int, networks ...string) []*DataStreamState {
	return nil
}
func (fakeBackend) GetPingState() ([]*vnet.PingReport, error) { return nil, nil }
func (fakeBackend) PingDomain(network, domain string) (time.Duration, error) {
	return 0, nil
}
func (fakeBackend) MountLink(alias string, link configv2.LinkSpec) error { return nil }
func (fakeBackend) MountTunnel(tunnel configv2.TunnelSpec) error         { return nil }
func (fakeBackend) RequestStop()                                         {}
func (fakeBackend) SubscribeNetworkState(buf int) (<-chan NetworkStateEvent, func()) {
	return make(chan NetworkStateEvent), func() {}
}
func (fakeBackend) SubscribeServiceStatus(buf int) (<-chan ServiceStatusEvent, func()) {
	return make(chan ServiceStatusEvent), func() {}
}

var testMeta = Meta{
	Protocol:   1,
	Version:    "1.2.3-test",
	Commit:     "abc1234",
	Mode:       "normal",
	ConfigPath: "/tmp/test-config.json",
}

func startTestServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:0"
	}
	s := New(fakeBackend{}, cfg, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(s.Stop)
	if s.Addr() == "" {
		t.Fatal("Addr() empty after Start")
	}
	return s
}

func doRequest(t *testing.T, method, url, token string, mutate func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if mutate != nil {
		mutate(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func TestStartSyncBindFailure(t *testing.T) {
	// 先占用一个端口，再让服务器绑定同一地址，必须同步报错
	occupied := startTestServer(t, Config{Meta: testMeta})

	s := New(fakeBackend{}, Config{Listen: occupied.Addr(), Meta: testMeta}, nil)
	if err := s.Start(); err == nil {
		s.Stop()
		t.Fatal("expected bind error, got nil")
	}
}

func TestAuthToken(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	// 无 token → 401
	resp := doRequest(t, "GET", base+"/api/v1/status", "", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", resp.StatusCode)
	}

	// 错 token → 401
	resp = doRequest(t, "GET", base+"/api/v1/status", "wrong", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token: want 401, got %d", resp.StatusCode)
	}

	// 正确 token → 200
	resp = doRequest(t, "GET", base+"/api/v1/status", "t0ken", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("correct token: want 200, got %d", resp.StatusCode)
	}
}

func TestHostCheck(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	resp := doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Host = "evil.com"
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("evil host: want 403, got %d", resp.StatusCode)
	}

	resp = doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Host = "127.0.0.1"
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("localhost host: want 200, got %d", resp.StatusCode)
	}
}

func TestCORSAllowList(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	// 白名单命中 → 反射 Origin
	resp := doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Header.Set("Origin", "http://localhost:1420")
	})
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:1420" {
		t.Fatalf("allowed origin: want reflected ACAO, got %q", got)
	}

	// 未命中 → 不设 ACAO
	resp = doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Header.Set("Origin", "http://evil.example")
	})
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin: want no ACAO, got %q", got)
	}

	// 预检 → 204
	resp = doRequest(t, "OPTIONS", base+"/api/v1/status", "", func(r *http.Request) {
		r.Header.Set("Origin", "tauri://localhost")
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight: want 204, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "tauri://localhost" {
		t.Fatalf("preflight: want reflected ACAO, got %q", got)
	}
}

func TestStatusMeta(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/status", "t0ken", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var envelope struct {
		ErrCode int `json:"ErrCode"`
		Data    struct {
			Running    bool   `json:"running"`
			Mode       string `json:"mode"`
			Version    string `json:"version"`
			Commit     string `json:"commit"`
			Protocol   int    `json:"protocol"`
			UptimeMs   int64  `json:"uptimeMs"`
			ConfigPath string `json:"configPath"`
		} `json:"Data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	d := envelope.Data
	if !d.Running {
		t.Error("running should be true")
	}
	if d.Mode != testMeta.Mode || d.Version != testMeta.Version ||
		d.Commit != testMeta.Commit || d.Protocol != testMeta.Protocol ||
		d.ConfigPath != testMeta.ConfigPath {
		t.Errorf("meta mismatch: %+v", d)
	}
	if d.UptimeMs < 0 {
		t.Errorf("uptimeMs should be >= 0, got %d", d.UptimeMs)
	}
}

func TestStandaloneNoToken(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/status", "", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("standalone without token: want 200, got %d", resp.StatusCode)
	}
}

func TestWebSocketAuth(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	wsBase := fmt.Sprintf("ws://%s/api/v1/ws", s.Addr())

	// 无 token → 拒绝握手（401）
	_, resp, err := websocket.DefaultDialer.Dial(wsBase, nil)
	if err == nil {
		t.Fatal("ws without token: want handshake failure")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ws without token: want 401, got %v", resp.StatusCode)
	}
	resp.Body.Close()

	// token 正确但 Origin 非法 → 拒绝握手（403）
	header := http.Header{"Origin": []string{"http://evil.example"}}
	_, resp, err = websocket.DefaultDialer.Dial(wsBase+"?token=t0ken", header)
	if err == nil {
		t.Fatal("ws with bad origin: want handshake failure")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("ws with bad origin: want 403, got %v", resp.StatusCode)
	}
	resp.Body.Close()

	// token 正确且 Origin 合法 → 握手成功
	header = http.Header{"Origin": []string{"http://localhost:1420"}}
	conn, resp, err := websocket.DefaultDialer.Dial(wsBase+"?token=t0ken", header)
	if err != nil {
		t.Fatalf("ws with token: %v", err)
	}
	resp.Body.Close()
	conn.Close()
}
