package configv2

import (
	"os"
	"path/filepath"
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

func findError(errs *ValidationErrors, code string) *ValidationError {
	for i, e := range errs.Errors {
		if e.Code == code {
			return &errs.Errors[i]
		}
	}
	return nil
}

func TestLoadAndCheck_SkipsBadTunnel(t *testing.T) {
	path := writeConfigFile(t, `{
  "tunnels": [
    {
      "id": "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
      "name": "good",
      "listen": "tcp://127.0.0.1:18081",
      "target": "tcp://127.0.0.1:18080"
    },
    {
      "id": "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce",
      "name": "bad",
      "listen": "://not-a-url",
      "target": "tcp://127.0.0.1:18082"
    }
  ]
}`)

	cfg, errs := LoadAndCheck(path)
	if cfg == nil {
		t.Fatalf("expected partial config, got nil (errs=%v)", errs)
	}
	if len(cfg.Tunnels) != 1 || cfg.Tunnels[0].Name != "good" {
		t.Fatalf("expected only the good tunnel to survive, got %+v", cfg.Tunnels)
	}
	if e := findError(errs, ErrInvalidItem); e == nil || !strings.Contains(e.Message, `"bad"`) {
		t.Fatalf("expected ERR_INVALID_ITEM naming the bad tunnel, got %v", errs)
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

func TestLoadAndCheck_CollectsValidateErrors(t *testing.T) {
	path := writeConfigFile(t, `{
  "tunnels": [
    {
      "id": "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
      "name": "rule-a",
      "listen": "tcp://127.0.0.1:18081",
      "target": "tcp://127.0.0.1:18080"
    },
    {
      "id": "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce",
      "name": "rule-b",
      "listen": "tcp://127.0.0.1:18081",
      "target": "tcp://127.0.0.1:18082"
    }
  ]
}`)

	// 冲突项保留在配置中（交由运行时隔离），校验错误里指明双方
	cfg, errs := LoadAndCheck(path)
	if cfg == nil || len(cfg.Tunnels) != 2 {
		t.Fatalf("expected both tunnels kept for runtime isolation, got %+v", cfg)
	}
	e := findError(errs, ErrListenConflict)
	if e == nil {
		t.Fatalf("expected ERR_LISTEN_CONFLICT, got %v", errs)
	}
	if !strings.Contains(e.Message, `"rule-a"`) || !strings.Contains(e.Message, `"rule-b"`) {
		t.Fatalf("conflict message should name both tunnels, got: %s", e.Message)
	}
}

func TestLoadAndCheck_Clean(t *testing.T) {
	path := writeConfigFile(t, `{
  "tunnels": [
    {
      "id": "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
      "name": "good",
      "listen": "tcp://127.0.0.1:18081",
      "target": "tcp://127.0.0.1:18080"
    }
  ]
}`)

	cfg, errs := LoadAndCheck(path)
	if cfg == nil || len(cfg.Tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %+v", cfg)
	}
	if len(errs.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}
