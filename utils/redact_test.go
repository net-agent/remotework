package utils

import "testing"

func TestRedactURL(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "authcode",
			raw:  "vtcp://host.link:3389?authcode=mysecret",
			want: "vtcp://host.link:3389?authcode=***",
		},
		{
			name: "authcodeRef among other params",
			raw:  "tcp://a:1?x=1&authcodeRef=ENV_KEY",
			want: "tcp://a:1?x=1&authcodeRef=***",
		},
		{
			name: "userinfo password",
			raw:  "socks5://user:pass@local:1080",
			want: "socks5://user:***@local:1080",
		},
		{
			name: "no sensitive fields",
			raw:  "tcp://127.0.0.1:8080",
			want: "tcp://127.0.0.1:8080",
		},
		{
			name: "unparseable still masked",
			raw:  "://bad?authcode=xyz",
			want: "://bad?authcode=***",
		},
		{
			name: "both userinfo and query",
			raw:  "socks5://user:pass@local:1080?secret=abc",
			want: "socks5://user:***@local:1080?secret=***",
		},
		{
			name: "token key",
			raw:  "http://127.0.0.1:8080/api/v1/stop?token=abc123",
			want: "http://127.0.0.1:8080/api/v1/stop?token=***",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RedactURL(c.raw); got != c.want {
				t.Errorf("RedactURL(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

func TestRedactText(t *testing.T) {
	raw := `dial wss://user:pass@relay.example.com:443 failed: ` +
		`GET https://relay/api?token=tok_123&authcode=psk&x=1`
	want := `dial wss://user:***@relay.example.com:443 failed: ` +
		`GET https://relay/api?token=***&authcode=***&x=1`
	if got := RedactText(raw); got != want {
		t.Fatalf("RedactText() = %q, want %q", got, want)
	}

	// 非 URL 的错误文本也要覆盖敏感键
	if got := RedactText("invalid config: password=hunter2"); got != "invalid config: password=***" {
		t.Fatalf("RedactText() = %q", got)
	}
}
