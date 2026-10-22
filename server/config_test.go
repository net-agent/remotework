package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid",
			cfg: Config{Server: ServerInfo{
				Listen:   "127.0.0.1:2040",
				Password: "dev_test_pswd",
			}},
		},
		{
			name: "valid ws",
			cfg: Config{Server: ServerInfo{
				Listen:   "127.0.0.1:2040",
				Password: "dev_test_pswd",
				WsEnable: true,
				WsPath:   "/wsconn",
			}},
		},
		{
			name:    "empty listen",
			cfg:     Config{Server: ServerInfo{Password: "dev_test_pswd"}},
			wantErr: true,
		},
		{
			name:    "invalid listen",
			cfg:     Config{Server: ServerInfo{Listen: "localhost", Password: "dev_test_pswd"}},
			wantErr: true,
		},
		{
			name:    "empty password",
			cfg:     Config{Server: ServerInfo{Listen: "127.0.0.1:2040"}},
			wantErr: true,
		},
		{
			name:    "short password",
			cfg:     Config{Server: ServerInfo{Listen: "127.0.0.1:2040", Password: "short"}},
			wantErr: true,
		},
		{
			name: "ws path missing",
			cfg: Config{Server: ServerInfo{
				Listen:   "127.0.0.1:2040",
				Password: "dev_test_pswd",
				WsEnable: true,
			}},
			wantErr: true,
		},
		{
			name: "ws path invalid",
			cfg: Config{Server: ServerInfo{
				Listen:   "127.0.0.1:2040",
				Password: "dev_test_pswd",
				WsEnable: true,
				WsPath:   "wsconn",
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewConfigRejectsUnsafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	data := []byte(`{"server":{"listen":"127.0.0.1:2040"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfig(path); err == nil {
		t.Fatal("expected empty password to be rejected")
	}
}
