package agent

import (
	"encoding/json"
	"testing"
)

func TestConfig_PreProcess_AgentURL(t *testing.T) {
	cfg := &Config{
		Agents: []AgentInfo{
			{URL: "vtcp://mydomain:mypass@1.2.3.4:8080"},
		},
	}
	if err := cfg.PreProcess(); err != nil {
		t.Fatalf("PreProcess() error: %v", err)
	}
	ag := cfg.Agents[0]
	if ag.Name != "vtcp" {
		t.Errorf("Name = %q, want %q", ag.Name, "vtcp")
	}
	if ag.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", ag.Protocol, "tcp")
	}
	if ag.Domain != "mydomain" {
		t.Errorf("Domain = %q, want %q", ag.Domain, "mydomain")
	}
	if ag.Password != "mypass" {
		t.Errorf("Password = %q, want %q", ag.Password, "mypass")
	}
	if ag.Address != "1.2.3.4:8080" {
		t.Errorf("Address = %q, want %q", ag.Address, "1.2.3.4:8080")
	}
}

func TestConfig_PreProcess_AgentURL_NoPassword(t *testing.T) {
	cfg := &Config{
		Agents: []AgentInfo{
			{URL: "vtcp://mydomain@1.2.3.4:8080"},
		},
	}
	if err := cfg.PreProcess(); err != nil {
		t.Fatalf("PreProcess() error: %v", err)
	}
	ag := cfg.Agents[0]
	if ag.Password != "" {
		t.Errorf("Password = %q, want empty", ag.Password)
	}
}

func TestConfig_PreProcess_AgentMap(t *testing.T) {
	agentMap, _ := json.Marshal(map[string]string{
		"mynet": "tcp://user:pass@server:9000/wspath",
	})
	cfg := &Config{
		AgentMap: agentMap,
	}
	if err := cfg.PreProcess(); err != nil {
		t.Fatalf("PreProcess() error: %v", err)
	}
	if len(cfg.Agents) != 1 {
		t.Fatalf("len(Agents) = %d, want 1", len(cfg.Agents))
	}
	ag := cfg.Agents[0]
	if ag.Name != "mynet" {
		t.Errorf("Name = %q, want %q", ag.Name, "mynet")
	}
	if ag.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", ag.Protocol, "tcp")
	}
	if ag.Domain != "user" {
		t.Errorf("Domain = %q, want %q", ag.Domain, "user")
	}
	if ag.Password != "pass" {
		t.Errorf("Password = %q, want %q", ag.Password, "pass")
	}
	if ag.Address != "server:9000" {
		t.Errorf("Address = %q, want %q", ag.Address, "server:9000")
	}
	if ag.WsPath != "/wspath" {
		t.Errorf("WsPath = %q, want %q", ag.WsPath, "/wspath")
	}
	if cfg.AgentMap != nil {
		t.Error("AgentMap should be nil after PreProcess")
	}
}

func TestConfig_PreProcess_PipeMap(t *testing.T) {
	pipeMap, _ := json.Marshal(map[string]PortproxyInfo{
		"mypipe": {ListenURL: "vtcp://0:80", TargetURL: "tcp://0:81"},
	})
	cfg := &Config{
		PipeMap: pipeMap,
	}
	if err := cfg.PreProcess(); err != nil {
		t.Fatalf("PreProcess() error: %v", err)
	}
	if len(cfg.Portproxy) != 1 {
		t.Fatalf("len(Portproxy) = %d, want 1", len(cfg.Portproxy))
	}
	pp := cfg.Portproxy[0]
	if pp.LogName != "mypipe" {
		t.Errorf("LogName = %q, want %q", pp.LogName, "mypipe")
	}
	if pp.ListenURL != "vtcp://0:80" {
		t.Errorf("ListenURL = %q, want %q", pp.ListenURL, "vtcp://0:80")
	}
	if pp.TargetURL != "tcp://0:81" {
		t.Errorf("TargetURL = %q, want %q", pp.TargetURL, "tcp://0:81")
	}
	if cfg.PipeMap != nil {
		t.Error("PipeMap should be nil after PreProcess")
	}
}

func TestConfig_PreProcess_SocksMap(t *testing.T) {
	socksMap, _ := json.Marshal(map[string]Socks5Info{
		"mysox": {ListenURL: "vtcp://0:1080", Username: "u", Password: "p"},
	})
	cfg := &Config{
		SocksMap: socksMap,
	}
	if err := cfg.PreProcess(); err != nil {
		t.Fatalf("PreProcess() error: %v", err)
	}
	if len(cfg.Socks5) != 1 {
		t.Fatalf("len(Socks5) = %d, want 1", len(cfg.Socks5))
	}
	sox := cfg.Socks5[0]
	if sox.LogName != "mysox" {
		t.Errorf("LogName = %q, want %q", sox.LogName, "mysox")
	}
	if sox.Username != "u" || sox.Password != "p" {
		t.Errorf("credentials = %q/%q, want u/p", sox.Username, sox.Password)
	}
	if cfg.SocksMap != nil {
		t.Error("SocksMap should be nil after PreProcess")
	}
}

func TestConfig_PreProcess_Empty(t *testing.T) {
	cfg := &Config{}
	if err := cfg.PreProcess(); err != nil {
		t.Fatalf("PreProcess() on empty config error: %v", err)
	}
}

func TestConfig_PreProcess_InvalidAgentMap(t *testing.T) {
	cfg := &Config{
		AgentMap: json.RawMessage(`{invalid`),
	}
	err := cfg.PreProcess()
	if err == nil {
		t.Fatal("expected error for invalid agent map JSON")
	}
}

func TestConfig_PreProcess_InvalidPipeMap(t *testing.T) {
	cfg := &Config{
		PipeMap: json.RawMessage(`{invalid`),
	}
	err := cfg.PreProcess()
	if err == nil {
		t.Fatal("expected error for invalid pipe map JSON")
	}
}

func TestConfig_PreProcess_InvalidSocksMap(t *testing.T) {
	cfg := &Config{
		SocksMap: json.RawMessage(`{invalid`),
	}
	err := cfg.PreProcess()
	if err == nil {
		t.Fatal("expected error for invalid socks map JSON")
	}
}

func TestConfig_PreProcess_MergesWithExisting(t *testing.T) {
	pipeMap, _ := json.Marshal(map[string]PortproxyInfo{
		"pipe2": {ListenURL: "tcp://0:82", TargetURL: "tcp://0:83"},
	})
	cfg := &Config{
		Portproxy: []PortproxyInfo{
			{ListenURL: "tcp://0:80", TargetURL: "tcp://0:81", LogName: "pipe1"},
		},
		PipeMap: pipeMap,
	}
	if err := cfg.PreProcess(); err != nil {
		t.Fatalf("PreProcess() error: %v", err)
	}
	if len(cfg.Portproxy) != 2 {
		t.Fatalf("len(Portproxy) = %d, want 2", len(cfg.Portproxy))
	}
}
