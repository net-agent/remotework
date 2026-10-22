package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type jsonFixture struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	N    int    `json:"n"`
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestLoadJSONFile_Comments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    jsonFixture
	}{
		{
			name: "full line and trailing line comments",
			content: `{
  // full line
  "name": "hello", // trailing
  "n": 1
}`,
			want: jsonFixture{Name: "hello", N: 1},
		},
		{
			name: "comment markers inside strings are preserved",
			content: `{
  "name": "http://example.com//path",
  "url": "https://example.com/*not-a-comment*/"
}`,
			want: jsonFixture{Name: "http://example.com//path", URL: "https://example.com/*not-a-comment*/"},
		},
		{
			name: "block comments",
			content: `{
  /* block
     comment */
  "name": "ok"
}`,
			want: jsonFixture{Name: "ok"},
		},
		{
			name: "escaped quote before comment marker",
			content: `{
  "name": "quote \" // still string",
  "n": 2
}`,
			want: jsonFixture{Name: `quote " // still string`, N: 2},
		},
		{
			name: "comment at eof without newline",
			content: `{"name":"eof"}
// comment`,
			want: jsonFixture{Name: "eof"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempFile(t, "config.json", tt.content)
			var got jsonFixture
			if err := LoadJSONFile(path, &got); err != nil {
				t.Fatalf("LoadJSONFile: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStripJSONCComments_PreservesLengthAndLines(t *testing.T) {
	src := []byte("{\n  // comment\n  \"a\": 1\n}")
	out := stripJSONCComments(src)
	if len(out) != len(src) {
		t.Fatalf("length changed: %d -> %d", len(src), len(out))
	}
	if bytes.Count(out, []byte("\n")) != bytes.Count(src, []byte("\n")) {
		t.Fatalf("newlines changed: %q -> %q", src, out)
	}
}

func TestLoadJSONFile_RejectsUnknownField(t *testing.T) {
	path := writeTempFile(t, "config.json", `{
  "name": "x",
  "trace": {"enable": true}
}`)
	var got jsonFixture
	err := LoadJSONFile(path, &got)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), `unknown field "trace"`) {
		t.Fatalf("error should name unknown field, got %v", err)
	}
}

func TestLoadJSONFile_RejectsTrailingData(t *testing.T) {
	path := writeTempFile(t, "config.json", `{"name":"one"} {"name":"two"}`)
	var got jsonFixture
	err := LoadJSONFile(path, &got)
	if err == nil {
		t.Fatal("expected trailing data error")
	}
	if !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("error should mention trailing data, got %v", err)
	}
}

func TestLoadTomlFile(t *testing.T) {
	type tomlFixture struct {
		Name string `toml:"name"`
	}

	t.Run("valid", func(t *testing.T) {
		path := writeTempFile(t, "config.toml", "name = \"ok\"\n")
		var got tomlFixture
		if err := LoadTomlFile(path, &got); err != nil {
			t.Fatalf("LoadTomlFile: %v", err)
		}
		if got.Name != "ok" {
			t.Fatalf("name = %q", got.Name)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		path := writeTempFile(t, "config.toml", "name = \"ok\"\ntrace = true\n")
		var got tomlFixture
		err := LoadTomlFile(path, &got)
		if err == nil {
			t.Fatal("expected unknown field error")
		}
		if !strings.Contains(err.Error(), "trace") || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("error should name unknown field, got %v", err)
		}
	})
}

func TestResolveConfigFile(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "agent.json")
	if err := os.WriteFile(explicit, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("explicit path", func(t *testing.T) {
		got, err := ResolveConfigFile(explicit)
		if err != nil || got != explicit {
			t.Fatalf("ResolveConfigFile(%q) = %q, %v", explicit, got, err)
		}
	})

	t.Run("fallback to config.json", func(t *testing.T) {
		fallback := filepath.Join(dir, "config.json")
		if err := os.WriteFile(fallback, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveConfigFile(filepath.Join(dir, "missing.json"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != fallback {
			t.Fatalf("got %q, want %q", got, fallback)
		}
	})

	t.Run("fallback to config.toml", func(t *testing.T) {
		tomlDir := t.TempDir()
		fallback := filepath.Join(tomlDir, "config.toml")
		if err := os.WriteFile(fallback, []byte(""), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveConfigFile(filepath.Join(tomlDir, "missing.json"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != fallback {
			t.Fatalf("got %q, want %q", got, fallback)
		}
	})

	t.Run("not found", func(t *testing.T) {
		emptyDir := t.TempDir()
		_, err := ResolveConfigFile(filepath.Join(emptyDir, "missing.json"))
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected not found error, got %v", err)
		}
	})
}

func TestFileExist(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !FileExist(file) {
		t.Fatal("existing file should be reported as existing")
	}
	if FileExist(filepath.Join(dir, "missing.json")) {
		t.Fatal("missing file should be reported as missing")
	}
	if FileExist(dir) {
		t.Fatal("directory should not be reported as an existing file")
	}
}
