package tunnel

import (
	"strings"
	"testing"

	"github.com/net-agent/remotework/configv2"
)

func vnetEndpoint(authcode, authcodeRef string) configv2.EndpointSpec {
	return configv2.EndpointSpec{
		Kind: configv2.EndpointVNet, Scheme: "vtcp",
		Host: "db.office", Port: "3306",
		VHostname: "db", Alias: "office",
		Authcode: authcode, AuthcodeRef: authcodeRef,
		RawURL: "vtcp://db.office:3306",
	}
}

func TestResolveEndpoint_VNetAuthcode(t *testing.T) {
	t.Run("plain authcode", func(t *testing.T) {
		ep, err := resolveEndpoint(vnetEndpoint("plain-code", ""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ep.authcode != "plain-code" || ep.network != "office" || ep.addr != "db:3306" {
			t.Fatalf("unexpected endpoint: %+v", ep)
		}
	})

	t.Run("authcodeRef resolved from env", func(t *testing.T) {
		t.Setenv("RW_TUNNEL_AUTH", "resolved-code")
		ep, err := resolveEndpoint(vnetEndpoint("", "env:RW_TUNNEL_AUTH"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ep.authcode != "resolved-code" {
			t.Fatalf("authcode = %q, want resolved-code", ep.authcode)
		}
	})

	t.Run("authcode wins when both are present", func(t *testing.T) {
		t.Setenv("RW_TUNNEL_AUTH", "resolved-code")
		ep, err := resolveEndpoint(vnetEndpoint("plain-code", "RW_TUNNEL_AUTH"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ep.authcode != "plain-code" {
			t.Fatalf("authcode = %q, want plain-code", ep.authcode)
		}
	})

	t.Run("missing authcodeRef fails closed", func(t *testing.T) {
		_, err := resolveEndpoint(vnetEndpoint("", "RW_TUNNEL_MISSING"))
		if err == nil {
			t.Fatal("expected error for missing authcodeRef")
		}
		if strings.Contains(err.Error(), "RW_TUNNEL_MISSING") {
			// 引用名可以出现在错误里，但绝不能作为密钥返回；这里主要断言失败关闭。
			t.Logf("error mentions ref name (allowed for diagnostics): %v", err)
		}
	})

	t.Run("no authcode and no ref rejected", func(t *testing.T) {
		_, err := resolveEndpoint(vnetEndpoint("", ""))
		if err == nil {
			t.Fatal("expected error for missing authcode/authcodeRef")
		}
	})
}

func TestResolveEndpoint_PhysicalAuthcodeRejected(t *testing.T) {
	tests := []struct {
		name string
		ep   configv2.EndpointSpec
	}{
		{
			name: "plain authcode",
			ep: configv2.EndpointSpec{
				Kind: configv2.EndpointNet, Scheme: "tcp",
				Host: "127.0.0.1", Port: "3306",
				Authcode: "fake-secret",
				RawURL:   "tcp://127.0.0.1:3306?authcode=fake-secret",
			},
		},
		{
			name: "authcodeRef",
			ep: configv2.EndpointSpec{
				Kind: configv2.EndpointNet, Scheme: "tcp",
				Host: "127.0.0.1", Port: "3306",
				AuthcodeRef: "PHYSICAL_SECRET",
				RawURL:      "tcp://127.0.0.1:3306?authcodeRef=PHYSICAL_SECRET",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveEndpoint(tt.ep)
			if err == nil {
				t.Fatal("expected physical endpoint authcode error")
			}
			if strings.Contains(err.Error(), "fake-secret") {
				t.Fatalf("error leaked authcode: %v", err)
			}
		})
	}
}

func TestResolveEndpoint_PhysicalAddr(t *testing.T) {
	ep, err := resolveEndpoint(configv2.EndpointSpec{
		Kind: configv2.EndpointNet, Scheme: "tcp",
		Host: "127.0.0.1", Port: "3306",
		RawURL: "tcp://127.0.0.1:3306",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.network != "tcp" || ep.addr != "127.0.0.1:3306" || ep.authcode != "" {
		t.Fatalf("unexpected endpoint: %+v", ep)
	}
}

func TestInternalURL_DoesNotEmbedAuthcodeRef(t *testing.T) {
	u := internalURL(vnetEndpoint("", "RW_TUNNEL_MISSING"))
	if strings.Contains(u, "authcode") || strings.Contains(u, "RW_TUNNEL_MISSING") {
		t.Fatalf("internal URL must not embed unresolved ref: %q", u)
	}

	u = internalURL(vnetEndpoint("plain-code", ""))
	if !strings.Contains(u, "authcode=plain-code") {
		t.Fatalf("plain authcode should be preserved for display: %q", u)
	}
}
