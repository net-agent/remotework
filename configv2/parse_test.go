package configv2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func writeRawConfigFile(t *testing.T, raw RawConfig) string {
	t.Helper()
	buf, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal raw config: %v", err)
	}
	return writeConfigFile(t, string(buf))
}

func tunnelRaw(id, name, listen, target string) TunnelRaw {
	return TunnelRaw{ID: id, Name: name, Listen: listen, Target: target}
}

func findError(errs *ValidationErrors, code string) *ValidationError {
	for i, e := range errs.Errors {
		if e.Code == code {
			return &errs.Errors[i]
		}
	}
	return nil
}

func hasErrorCode(errs *ValidationErrors, code string) bool {
	return findError(errs, code) != nil
}

func linkAliases(cfg *CanonicalConfig) []string {
	aliases := make([]string, 0, len(cfg.Links))
	for alias := range cfg.Links {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

func tunnelNames(cfg *CanonicalConfig) []string {
	names := make([]string, 0, len(cfg.Tunnels))
	for _, t := range cfg.Tunnels {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

func assertStringSlice(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	sort.Strings(want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}

// TestLoadAndCheck_PerItemTolerance 是逐项容错的核心表驱动测试：每个用例
// 混入坏项与好项，断言坏项被丢弃、好项保留、错误仍然返回。
func TestLoadAndCheck_PerItemTolerance(t *testing.T) {
	const (
		idA = "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
		idB = "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce"
	)

	goodLink := func(alias string) map[string]string {
		return map[string]string{alias: "tcp://relay.example.com:2040?as=agent1"}
	}

	tests := []struct {
		name        string
		raw         RawConfig
		wantLinks   []string
		wantTunnels []string
		wantCodes   []string
	}{
		{
			name: "bad link dropped and dependent vtcp tunnel dropped",
			raw: RawConfig{
				Links: map[string]string{
					"good": "tcp://relay.example.com:2040?as=agent1",
					"bad":  "tcp://relay.example.com:2040", // missing as
				},
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "dep-bad", "tcp://127.0.0.1:18081", "vtcp://db.bad:3306?authcode=x"),
					tunnelRaw(idB, "dep-good", "tcp://127.0.0.1:18082", "vtcp://db.good:3306?authcode=x"),
				},
			},
			wantLinks:   []string{"good"},
			wantTunnels: []string{"dep-good"},
			wantCodes:   []string{ErrMissingAs, ErrAliasNotFound},
		},
		{
			name: "link with invalid alias dropped, good link kept",
			raw: RawConfig{
				Links: map[string]string{
					"good":       "tcp://relay.example.com:2040?as=agent1",
					"bad alias!": "tcp://relay.example.com:2040?as=agent2",
				},
			},
			wantLinks: []string{"good"},
			wantCodes: []string{ErrInvalidAlias},
		},
		{
			name: "tunnel missing id dropped, good tunnel kept",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					tunnelRaw("", "no-id", "tcp://127.0.0.1:18081", "tcp://10.0.0.1:80"),
					tunnelRaw(idB, "good", "tcp://127.0.0.1:18082", "tcp://10.0.0.2:80"),
				},
			},
			wantLinks:   nil,
			wantTunnels: []string{"good"},
			wantCodes:   []string{ErrInvalidTunnelID},
		},
		{
			name: "tunnel missing name/listen/target dropped",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					{ID: idA},
					tunnelRaw(idB, "good", "tcp://127.0.0.1:18082", "tcp://10.0.0.2:80"),
				},
			},
			wantLinks:   nil,
			wantTunnels: []string{"good"},
			wantCodes:   []string{ErrMissingField},
		},
		{
			name: "duplicate tunnel name drops only the later one",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "same", "tcp://127.0.0.1:18081", "tcp://10.0.0.1:80"),
					tunnelRaw(idB, "same", "tcp://127.0.0.1:18082", "tcp://10.0.0.2:80"),
				},
			},
			wantLinks:   nil,
			wantTunnels: []string{"same"},
			wantCodes:   []string{ErrDuplicateTunnelName},
		},
		{
			name: "duplicate tunnel id drops only the later one",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "first", "tcp://127.0.0.1:18081", "tcp://10.0.0.1:80"),
					tunnelRaw(idA, "second", "tcp://127.0.0.1:18082", "tcp://10.0.0.2:80"),
				},
			},
			wantLinks:   nil,
			wantTunnels: []string{"first"},
			wantCodes:   []string{ErrDuplicateTunnelID},
		},
		{
			name: "duplicate listen drops only the later one",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "first", "tcp://127.0.0.1:18081", "tcp://10.0.0.1:80"),
					tunnelRaw(idB, "second", "tcp://127.0.0.1:18081", "tcp://10.0.0.2:80"),
				},
			},
			wantLinks:   nil,
			wantTunnels: []string{"first"},
			wantCodes:   []string{ErrListenConflict},
		},
		{
			name: "unknown scheme dropped",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "udp", "tcp://127.0.0.1:18081", "udp://10.0.0.1:53"),
					tunnelRaw(idB, "good", "tcp://127.0.0.1:18082", "tcp://10.0.0.2:80"),
				},
			},
			wantTunnels: []string{"good"},
			wantCodes:   []string{ErrInvalidScheme},
		},
		{
			name: "vtcp without authcode dropped",
			raw: RawConfig{
				Links: goodLink("office"),
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "no-auth", "tcp://127.0.0.1:18081", "vtcp://db.office:3306"),
					tunnelRaw(idB, "good", "tcp://127.0.0.1:18082", "vtcp://db.office:3306?authcode=x"),
				},
			},
			wantLinks:   []string{"office"},
			wantTunnels: []string{"good"},
			wantCodes:   []string{ErrVtcpAuthRequired},
		},
		{
			name: "physical endpoint authcode dropped",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "plaintext", "tcp://127.0.0.1:18081?authcode=fake", "tcp://10.0.0.1:80"),
					tunnelRaw(idB, "good", "tcp://127.0.0.1:18082", "tcp://10.0.0.2:80"),
				},
			},
			wantTunnels: []string{"good"},
			wantCodes:   []string{ErrAuthcodeNotAllowed},
		},
		{
			name: "open socks5 on non-loopback dropped",
			raw: RawConfig{
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "open-proxy", "tcp://0.0.0.0:1080", "socks5://"),
					tunnelRaw(idB, "good", "tcp://127.0.0.1:18082", "tcp://10.0.0.2:80"),
				},
			},
			wantTunnels: []string{"good"},
			wantCodes:   []string{ErrSocks5AuthRequired},
		},
		{
			name: "clean config keeps everything",
			raw: RawConfig{
				Links: map[string]string{
					"office": "tcp://relay.example.com:2040?as=agent1&auth=plain",
				},
				Tunnels: []TunnelRaw{
					tunnelRaw(idA, "good", "tcp://127.0.0.1:18081", "vtcp://db.office:3306?authcode=x"),
				},
			},
			wantLinks:   []string{"office"},
			wantTunnels: []string{"good"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeRawConfigFile(t, tt.raw)
			cfg, errs := LoadAndCheck(path)
			if cfg == nil {
				t.Fatalf("expected partial config, got nil (errs=%v)", errs)
			}

			assertStringSlice(t, "links", linkAliases(cfg), append([]string(nil), tt.wantLinks...))
			assertStringSlice(t, "tunnels", tunnelNames(cfg), append([]string(nil), tt.wantTunnels...))

			for _, code := range tt.wantCodes {
				if !hasErrorCode(errs, code) {
					t.Fatalf("expected error code %s, got %v", code, errs)
				}
			}
			if len(tt.wantCodes) == 0 && len(errs.Errors) != 0 {
				t.Fatalf("expected no errors, got %v", errs)
			}
		})
	}
}

func TestLoadAndCheck_ParseFailure(t *testing.T) {
	path := writeConfigFile(t, `{ not json`)

	cfg, errs := LoadAndCheck(path)
	if cfg != nil {
		t.Fatalf("expected nil config on parse failure, got %+v", cfg)
	}
	if findError(errs, ErrConfigParse) == nil {
		t.Fatalf("expected ERR_CONFIG_PARSE, got %v", errs)
	}
}

func TestLoadAndCheck_RejectsUnknownFields(t *testing.T) {
	path := writeConfigFile(t, `{
  "tunnels": [
    {
      "id": "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
      "name": "bad",
      "listen": "tcp://127.0.0.1:18081",
      "target": "tcp://127.0.0.1:18080",
      "trace": {"enable": true}
    }
  ]
}`)

	cfg, errs := LoadAndCheck(path)
	if cfg != nil {
		t.Fatalf("unknown field must reject the whole file, got %+v", cfg)
	}
	e := findError(errs, ErrConfigParse)
	if e == nil {
		t.Fatalf("expected ERR_CONFIG_PARSE, got %v", errs)
	}
	if !strings.Contains(e.Message, `unknown field "trace"`) {
		t.Fatalf("error should name the unknown field, got %q", e.Message)
	}
}

func TestLoadAndCheck_JSONCommentsAndCommentField(t *testing.T) {
	path := writeConfigFile(t, `{
  // 整行注释
  "links": {
    "office": "tcp://relay.example.com:2040?as=agent1" // 行尾注释
  },
  "tunnels": [
    {
      "id": "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
      "name": "with//slash",
      "comment": "see http://example.com//path /* not a comment */",
      "listen": "tcp://127.0.0.1:18081", // 注释
      "target": "vtcp://db.office:3306?authcode=x"
    }
  ]
}`)

	cfg, errs := LoadAndCheck(path)
	if cfg == nil {
		t.Fatalf("expected parsed config, got nil (errs=%v)", errs)
	}
	if len(errs.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(cfg.Links) != 1 || len(cfg.Tunnels) != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Tunnels[0].Name != "with//slash" {
		t.Fatalf("string content was mangled: %q", cfg.Tunnels[0].Name)
	}
}
