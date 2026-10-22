package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// serviceBackend 返回可控的服务状态（脱敏测试用）
type serviceBackend struct {
	fakeBackend
	states []*vservice.ServiceState
}

func (b *serviceBackend) GetAllServiceState() ([]*vservice.ServiceState, error) {
	return b.states, nil
}

// networkBackend 返回可控的网络报告（脱敏测试用）
type networkBackend struct {
	fakeBackend
	reports []vnet.NetworkReport
}

func (b *networkBackend) GetAllNetworkState() ([]vnet.NetworkReport, error) {
	return b.reports, nil
}

func makeSecretService() *vservice.ServiceState {
	svc := &vservice.ServiceState{
		Name:      "rdp",
		Type:      "portproxy",
		ListenURL: "vtcp://my-pc.link1:13389?authcode=LISTEN_SECRET",
		TargetURL: "vtcp://office-pc.link2:3389?authcode=TARGET_SECRET",
	}
	svc.SetStatus(vservice.StatusRunning)
	svc.SetLastErr(errors.New("dial vtcp://x.link:1?authcode=ERR_SECRET failed"))
	return svc
}

func assertNoSecrets(t *testing.T, context string, raw string) {
	t.Helper()
	for _, secret := range []string{"LISTEN_SECRET", "TARGET_SECRET", "ERR_SECRET", "USERINFO_SECRET"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("%s leaks %s: %s", context, secret, raw)
		}
	}
}

func TestToServiceDTORedactsSecrets(t *testing.T) {
	dto := toServiceDTO(makeSecretService())
	if !strings.Contains(dto.ListenURL, "authcode=***") {
		t.Errorf("listenURL should be redacted, got %q", dto.ListenURL)
	}
	if !strings.Contains(dto.TargetURL, "authcode=***") {
		t.Errorf("targetURL should be redacted, got %q", dto.TargetURL)
	}
	if !strings.Contains(dto.LastErr, "authcode=***") {
		t.Errorf("lastErr should be redacted, got %q", dto.LastErr)
	}
	assertNoSecrets(t, "ServiceStateDTO", fmt.Sprintf("%+v", dto))
}

func TestToNetworkDTORedactsUserinfo(t *testing.T) {
	dto := toNetworkDTO(vnet.NetworkReport{
		Name:     "link1",
		Address:  "wss://user:USERINFO_SECRET@relay.example:443",
		LastErr:  "register wss://user:USERINFO_SECRET@relay.example?authcode=ERR_SECRET failed",
		Domain:   "my-pc",
		State:    "running",
		Protocol: "wss",
	})
	if strings.Contains(dto.Address, "USERINFO_SECRET") {
		t.Fatalf("address leaks userinfo password: %q", dto.Address)
	}
	if !strings.Contains(dto.Address, "user:***@") {
		t.Errorf("address should keep user but mask password, got %q", dto.Address)
	}
	assertNoSecrets(t, "NetworkStateDTO", fmt.Sprintf("%+v", dto))
}

// TestServicesEndpointRedactsSecrets REST /services 不能返回 authcode
func TestServicesEndpointRedactsSecrets(t *testing.T) {
	b := &serviceBackend{states: []*vservice.ServiceState{makeSecretService()}}
	s := startTestServerWith(t, b, Config{Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/services", "", nil)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	assertNoSecrets(t, "GET /services", string(body))
	if !strings.Contains(string(body), "authcode=***") {
		t.Errorf("GET /services should still expose redacted url: %s", body)
	}
}

// TestNetworksEndpointRedactsUserinfo REST /networks 不能返回 relay userinfo 口令
func TestNetworksEndpointRedactsUserinfo(t *testing.T) {
	b := &networkBackend{reports: []vnet.NetworkReport{{
		Name:     "link1",
		Address:  "wss://user:USERINFO_SECRET@relay.example:443",
		State:    "running",
		Protocol: "wss",
	}}}
	s := startTestServerWith(t, b, Config{Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/networks", "", nil)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	assertNoSecrets(t, "GET /networks", string(body))
}

// TestWSServiceSnapshotRedactsSecrets WS service.snapshot 走同一 DTO，不能泄露 authcode
func TestWSServiceSnapshotRedactsSecrets(t *testing.T) {
	b := &serviceBackend{states: []*vservice.ServiceState{makeSecretService()}}
	s := startTestServerWith(t, b, Config{Meta: testMeta})
	conn := dialTestWS(t, s)

	if err := conn.WriteJSON(map[string]interface{}{
		"action": "subscribe",
		"topics": []string{"service"},
	}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting service.snapshot")
		}
		evt := readWSMessage(t, conn)
		if evt.Type != "service.snapshot" {
			continue
		}
		raw, err := json.Marshal(evt.Data)
		if err != nil {
			t.Fatalf("marshal snapshot: %v", err)
		}
		assertNoSecrets(t, "service.snapshot", string(raw))
		if !strings.Contains(string(raw), "authcode=***") {
			t.Errorf("snapshot should carry redacted url: %s", raw)
		}
		return
	}
}

// TestPollerServiceStatsRedactsSecrets service.stats 同样必须脱敏
func TestPollerServiceStatsRedactsSecrets(t *testing.T) {
	b := &serviceBackend{states: []*vservice.ServiceState{makeSecretService()}}
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	poller := NewStatePoller(b, hub, nil, nil, time.Hour, time.Hour)

	c := &WSClient{send: make(chan []byte, 8)}
	hub.addClient(c)

	poller.pollServiceStats()
	evt := readWSEvent(t, c)
	if evt.Type != "service.stats" {
		t.Fatalf("want service.stats, got %q", evt.Type)
	}
	raw, err := json.Marshal(evt.Data)
	if err != nil {
		t.Fatalf("marshal stats: %v", err)
	}
	assertNoSecrets(t, "service.stats", string(raw))
}

// TestStatusConfigErrorRedacted /status 保留 ConfigPath，但 ConfigError 必须脱敏
func TestStatusConfigErrorRedacted(t *testing.T) {
	meta := testMeta
	meta.ConfigError = "load link: wss://user:USERINFO_SECRET@relay.example?authcode=ERR_SECRET invalid"
	s := startTestServer(t, Config{Meta: meta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/status", "", nil)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	assertNoSecrets(t, "GET /status", string(body))
	if !strings.Contains(string(body), testMeta.ConfigPath) {
		t.Errorf("configPath should be kept, body=%s", body)
	}

	// WS hub.snapshot 使用同一字段，同样不得泄露
	wsConn := dialTestWS(t, s)
	if err := wsConn.WriteJSON(map[string]interface{}{"action": "subscribe", "topics": []string{"hub"}}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
	for {
		evt := readWSMessage(t, wsConn)
		if evt.Type != "hub.snapshot" {
			continue
		}
		raw, err := json.Marshal(evt.Data)
		if err != nil {
			t.Fatalf("marshal hub snapshot: %v", err)
		}
		assertNoSecrets(t, "hub.snapshot", string(raw))
		return
	}
}
