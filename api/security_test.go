package api

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// stopBackend 记录 RequestStop 调用次数（fakeBackend 的可观测变体）
type stopBackend struct {
	fakeBackend
	stops atomic.Int32
}

func (b *stopBackend) RequestStop() { b.stops.Add(1) }

func waitStops(t *testing.T, b *stopBackend, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b.stops.Load() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("stops = %d, want %d", b.stops.Load(), want)
}

// TestCSRFBlocksCrossOriginWrites 回归 P0-3：
// 带 Origin 的写请求未命中白名单必须直接 403（不能只省略 ACAO 头），
// 无 Origin 的非浏览器请求（CLI/sidecar）仍放行，安全方法不受影响。
func TestCSRFBlocksCrossOriginWrites(t *testing.T) {
	b := &stopBackend{}
	s := startTestServerWith(t, b, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	// 未命中白名单的跨站 POST → 403，且绝不触发后端动作
	resp := doRequest(t, "POST", base+"/api/v1/stop", "t0ken", func(r *http.Request) {
		r.Header.Set("Origin", "http://evil.example")
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin POST: want 403, got %d", resp.StatusCode)
	}
	if got := b.stops.Load(); got != 0 {
		t.Fatalf("cross-origin POST must not reach backend, stops=%d", got)
	}

	// 无 Origin（非浏览器客户端）→ 允许
	resp = doRequest(t, "POST", base+"/api/v1/stop", "t0ken", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("no-origin POST: want 200, got %d", resp.StatusCode)
	}
	waitStops(t, b, 1)

	// 白名单 Origin → 允许
	resp = doRequest(t, "POST", base+"/api/v1/stop", "t0ken", func(r *http.Request) {
		r.Header.Set("Origin", "http://localhost:1420")
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("allowed-origin POST: want 200, got %d", resp.StatusCode)
	}
	waitStops(t, b, 2)

	// 安全方法即使 Origin 未命中也不拒绝（但不得反射 ACAO）
	resp = doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Header.Set("Origin", "http://evil.example")
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cross-origin GET: want 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("cross-origin GET must not get ACAO, got %q", got)
	}
}

// TestCSRFBlocksStandaloneWrites standalone（无 token）模式同样必须拦截跨站写请求
func TestCSRFBlocksStandaloneWrites(t *testing.T) {
	b := &stopBackend{}
	s := startTestServerWith(t, b, Config{Meta: testMeta})
	base := "http://" + s.Addr()

	resp := doRequest(t, "POST", base+"/api/v1/stop", "", func(r *http.Request) {
		r.Header.Set("Origin", "http://evil.example")
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("standalone cross-origin POST: want 403, got %d", resp.StatusCode)
	}
	if got := b.stops.Load(); got != 0 {
		t.Fatalf("standalone cross-origin POST reached backend, stops=%d", got)
	}
}

// TestTokenConstantTimeEdgeCases 校验 token 比较的边界：多空格/大小写前缀均拒绝
func TestTokenConstantTimeEdgeCases(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	for _, auth := range []string{
		"Bearer  t0ken", // 双空格
		"bearer t0ken",  // 小写前缀
		"t0ken",
	} {
		resp := doRequest(t, "GET", base+"/api/v1/status", "", func(r *http.Request) {
			r.Header.Set("Authorization", auth)
		})
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("auth %q: want 401, got %d", auth, resp.StatusCode)
		}
	}
}

// TestStandaloneNonLoopbackRefused 无 token 且实际监听非 loopback → Start 必须报错并关闭 listener
func TestStandaloneNonLoopbackRefused(t *testing.T) {
	s := New(fakeBackend{}, Config{Listen: "0.0.0.0:0", Meta: testMeta}, nil)
	if err := s.Start(); err == nil {
		s.Stop()
		t.Fatal("expected error for token-less non-loopback listener")
	}
	if got := s.Addr(); got != "" {
		t.Fatalf("failed Start should not publish addr, got %q", got)
	}
	s.Stop() // 未启动的 Stop 也应安全
}

// TestWSBearerAuth WS 除 ?token= 外也接受 Authorization: Bearer（桌面端可用标准头）
func TestWSBearerAuth(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	wsURL := fmt.Sprintf("ws://%s/api/v1/ws", s.Addr())

	// 错误 Bearer → 401
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer wrong"},
	})
	if err == nil {
		t.Fatal("wrong bearer should fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("wrong bearer: want 401, got %d", status)
	}
	resp.Body.Close()

	// 正确 Bearer（无 query token）→ 握手成功
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer t0ken"},
	})
	if err != nil {
		t.Fatalf("bearer ws dial: %v", err)
	}
	resp.Body.Close()
	conn.Close()

	// 兼容：query token 仍可用
	conn, resp, err = websocket.DefaultDialer.Dial(wsURL+"?token=t0ken", nil)
	if err != nil {
		t.Fatalf("query-token ws dial: %v", err)
	}
	resp.Body.Close()
	conn.Close()
}
