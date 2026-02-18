package agent

import (
	"strings"
	"testing"
)

func TestServiceManager_GetAllState_Empty(t *testing.T) {
	sm := NewServiceManager(nil)
	_, err := sm.GetAllState()
	if err == nil {
		t.Fatal("expected error for empty service manager")
	}
}

func TestServiceManager_GetAllState(t *testing.T) {
	sm := NewServiceManager(nil)
	svc1, _ := newTestService("svc1", "portproxy", "tcp://0:80", "tcp://0:81")
	svc2, _ := newTestService("svc2", "socks5", "vtcp://0:1080", "")
	sm.Add(svc1)
	sm.Add(svc2)

	states, err := sm.GetAllState()
	if err != nil {
		t.Fatalf("GetAllState() error: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("len = %d, want 2", len(states))
	}
	if states[0].Name != "svc1" {
		t.Errorf("states[0].Name = %q, want %q", states[0].Name, "svc1")
	}
	if states[1].Name != "svc2" {
		t.Errorf("states[1].Name = %q, want %q", states[1].Name, "svc2")
	}
}

func TestServiceManager_GetAllStateString(t *testing.T) {
	sm := NewServiceManager(nil)
	svc, _ := newTestService("svc1", "portproxy", "tcp://0:80", "tcp://0:81")
	sm.Add(svc)

	s := sm.GetAllStateString()
	if !strings.Contains(s, "report service") {
		t.Errorf("output missing header, got: %q", s)
	}
	if !strings.Contains(s, "svc1") {
		t.Errorf("output missing service name, got: %q", s)
	}
}

func TestServiceManager_GetAllStateString_Empty(t *testing.T) {
	sm := NewServiceManager(nil)
	s := sm.GetAllStateString()
	if !strings.Contains(s, "NO SERVICES") {
		t.Errorf("expected NO SERVICES message, got: %q", s)
	}
}

func TestNetworkRegistry_GetAllState_Empty(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	_, err := nr.GetAllState()
	if err == nil {
		t.Fatal("expected error for empty registry")
	}
}

func TestNetworkRegistry_GetAllState(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	mn := newMockNetwork("vnet")
	mn.report = NetworkReport{Name: "vnet", State: "online"}
	nr.Add(mn)

	reports, err := nr.GetAllState()
	if err != nil {
		t.Fatalf("GetAllState() error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("len = %d, want 1", len(reports))
	}
	if reports[0].Name != "vnet" {
		t.Errorf("Name = %q, want %q", reports[0].Name, "vnet")
	}
}

func TestNetworkRegistry_GetAllStateString(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	mn := newMockNetwork("vnet")
	mn.report = NetworkReport{Name: "vnet", Address: "1.2.3.4:80", Domain: "test"}
	nr.Add(mn)

	s := nr.GetAllStateString()
	if !strings.Contains(s, "report network") {
		t.Errorf("output missing header, got: %q", s)
	}
	if !strings.Contains(s, "vnet") {
		t.Errorf("output missing network name, got: %q", s)
	}
}

func TestNetworkRegistry_GetAllStateString_Empty(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	s := nr.GetAllStateString()
	if !strings.Contains(s, "NO NETWORKS") {
		t.Errorf("expected NO NETWORKS message, got: %q", s)
	}
}

func TestHub_GetPingState_NoServices(t *testing.T) {
	hub := NewHub(nil)
	_, err := hub.GetPingState()
	if err == nil {
		t.Fatal("expected error for no services")
	}
}

func TestHub_GetPingState(t *testing.T) {
	hub := NewHub(nil)
	hub.AddNetwork(newMockNetwork("vnet"))

	svc1, _ := newTestService("pp1", "portproxy", "vnet://remote:80", "tcp://0:81")
	svc2, _ := newTestService("pp2", "portproxy", "tcp://0:82", "vnet://remote:83")
	hub.AddService(svc1)
	hub.AddService(svc2)

	reports, err := hub.GetPingState()
	if err != nil {
		t.Fatalf("GetPingState() error: %v", err)
	}
	if len(reports) == 0 {
		t.Fatal("expected at least one ping report")
	}

	// 验证至少有一个 report 包含 vnet 网络
	found := false
	for _, r := range reports {
		if r.Network == "vnet" {
			found = true
		}
	}
	if !found {
		t.Error("expected a report with Network=vnet")
	}
}

func TestHub_GetPingStateString(t *testing.T) {
	hub := NewHub(nil)
	// 无服务时
	s := hub.GetPingStateString()
	if !strings.Contains(s, "NO SERVICES") {
		t.Errorf("expected NO SERVICES, got: %q", s)
	}
}

func TestParseURLDepend(t *testing.T) {
	tests := []struct {
		raw        string
		wantScheme string
		wantHost   string
		wantErr    bool
	}{
		{"vtcp://remote:80", "vtcp", "remote", false},
		{"tcp://localhost:3389", "tcp", "localhost", false},
		{"ws://example.com:443/path", "ws", "example.com", false},
		{"", "", "", true},
	}
	for _, tt := range tests {
		scheme, host, err := parseURLDepend(tt.raw)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseURLDepend(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			continue
		}
		if scheme != tt.wantScheme {
			t.Errorf("parseURLDepend(%q) scheme = %q, want %q", tt.raw, scheme, tt.wantScheme)
		}
		if host != tt.wantHost {
			t.Errorf("parseURLDepend(%q) host = %q, want %q", tt.raw, host, tt.wantHost)
		}
	}
}

func TestParseDependAndSaveToMap(t *testing.T) {
	m := make(map[string]*PingReport)

	svc := &Service{
		ServiceState: ServiceState{
			Name:      "pp1",
			ListenURL: "vnet://remote:80",
			TargetURL: "tcp://localhost:81",
		},
	}
	parseDependAndSaveToMap(m, svc)

	// tcp 前缀应该被跳过
	for k := range m {
		if strings.HasPrefix(k, "tcp") {
			t.Errorf("tcp-prefixed key %q should not be in map", k)
		}
	}

	// 应该有 vnet 相关的条目
	if len(m) == 0 {
		t.Fatal("map should not be empty")
	}

	// 验证 vnet://remote 条目
	key := "vnet://remote"
	report, found := m[key]
	if !found {
		t.Fatalf("missing key %q in map, keys: %v", key, mapKeys(m))
	}
	if report.Network != "vnet" {
		t.Errorf("Network = %q, want %q", report.Network, "vnet")
	}
	if report.Domain != "remote" {
		t.Errorf("Domain = %q, want %q", report.Domain, "remote")
	}
	if len(report.UsedServices) != 1 || report.UsedServices[0] != "pp1.listen" {
		t.Errorf("UsedServices = %v, want [pp1.listen]", report.UsedServices)
	}
}

func TestParseDependAndSaveToMap_LocalDomain(t *testing.T) {
	m := make(map[string]*PingReport)

	svc := &Service{
		ServiceState: ServiceState{
			Name:      "pp1",
			ListenURL: "vnet://0:80",
			TargetURL: "vnet://local:81",
		},
	}
	parseDependAndSaveToMap(m, svc)

	// domain "0" 和 "local" 不应该创建 domain 级别的条目
	for k, r := range m {
		if r.Domain == "0" || r.Domain == "local" {
			t.Errorf("key %q has domain %q which should be skipped", k, r.Domain)
		}
	}
}

func mapKeys(m map[string]*PingReport) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
