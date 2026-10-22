package configv2

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize_Links(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		rawURL  string
		wantErr bool
		check   func(t *testing.T, spec LinkSpec)
	}{
		{
			name:   "basic tcp link",
			alias:  "office",
			rawURL: "tcp://relay.corp.com:7000?as=pc-01&auth=work-secret",
			check: func(t *testing.T, spec LinkSpec) {
				assertEqual(t, "Alias", spec.Alias, "office")
				assertEqual(t, "Scheme", spec.Scheme, "tcp")
				assertEqual(t, "RelayURL", spec.RelayURL, "tcp://relay.corp.com:7000")
				assertEqual(t, "As", spec.As, "pc-01")
				assertEqual(t, "Auth", spec.Auth, "work-secret")
				assertEqual(t, "AuthRef", spec.AuthRef, "")
				assertIntEqual(t, "Keepalive", spec.Keepalive, 15)
				if spec.Via != nil {
					t.Error("Via should be nil")
				}
			},
		},
		{
			name:   "wss link with authRef",
			alias:  "home",
			rawURL: "wss://tunnel.home.org/ws?as=macbook&authRef=HOME_RELAY_AUTH",
			check: func(t *testing.T, spec LinkSpec) {
				assertEqual(t, "Scheme", spec.Scheme, "wss")
				assertEqual(t, "As", spec.As, "macbook")
				assertEqual(t, "AuthRef", spec.AuthRef, "HOME_RELAY_AUTH")
				assertIntEqual(t, "Keepalive", spec.Keepalive, DefaultKeepalive)
				if spec.Via != nil {
					t.Error("Via should be nil")
				}
			},
		},
		{
			name:   "userinfo removed from relay URL",
			alias:  "home",
			rawURL: "wss://user:pass@tunnel.home.org/ws?as=macbook",
			check: func(t *testing.T, spec LinkSpec) {
				assertEqual(t, "RelayURL", spec.RelayURL, "wss://tunnel.home.org/ws")
			},
		},
		{
			name:    "explicit keepalive unsupported",
			alias:   "net1",
			rawURL:  "tcp://relay.com:5000?as=node1&keepalive=10",
			wantErr: true,
		},
		{
			name:    "via unsupported",
			alias:   "net1",
			rawURL:  "tcp://relay.com:5000?as=node1&via=gateway.office:7000",
			wantErr: true,
		},
		{
			name:   "default keepalive",
			alias:  "net1",
			rawURL: "tcp://relay.com:5000?as=node1",
			check: func(t *testing.T, spec LinkSpec) {
				assertIntEqual(t, "Keepalive", spec.Keepalive, 15)
			},
		},
		{
			name:    "keepalive rejected regardless of value validity",
			alias:   "net1",
			rawURL:  "tcp://relay.com:5000?as=node1&keepalive=abc",
			wantErr: true,
		},
		{
			name:    "via without port also rejected as unsupported",
			alias:   "net1",
			rawURL:  "tcp://relay.com:5000?as=node1&via=gateway.office",
			wantErr: true,
		},
		{
			name:    "via empty host also rejected as unsupported",
			alias:   "net1",
			rawURL:  "tcp://relay.com:5000?as=node1&via=:7000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &RawConfig{
				Links: map[string]string{tt.alias: tt.rawURL},
			}
			canonical, err := Normalize(raw)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			spec, ok := canonical.Links[tt.alias]
			if !ok {
				t.Fatalf("link %q not found", tt.alias)
			}
			tt.check(t, spec)
		})
	}
}

func TestNormalize_TunnelEndpoints(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
		check   func(t *testing.T, ep EndpointSpec)
	}{
		{
			name:   "tcp endpoint",
			rawURL: "tcp://127.0.0.1:13306",
			check: func(t *testing.T, ep EndpointSpec) {
				assertEqual(t, "Kind", ep.Kind.String(), "net")
				assertEqual(t, "Scheme", ep.Scheme, "tcp")
				assertEqual(t, "Host", ep.Host, "127.0.0.1")
				assertEqual(t, "Port", ep.Port, "13306")
				assertEqual(t, "VHostname", ep.VHostname, "")
				assertEqual(t, "Alias", ep.Alias, "")
			},
		},
		{
			name:   "vtcp endpoint",
			rawURL: "vtcp://db-server.office:3306",
			check: func(t *testing.T, ep EndpointSpec) {
				assertEqual(t, "Kind", ep.Kind.String(), "vnet")
				assertEqual(t, "Scheme", ep.Scheme, "vtcp")
				assertEqual(t, "Host", ep.Host, "db-server.office")
				assertEqual(t, "Port", ep.Port, "3306")
				assertEqual(t, "VHostname", ep.VHostname, "db-server")
				assertEqual(t, "Alias", ep.Alias, "office")
			},
		},
		{
			name:   "vtcp with multi-segment vhostname",
			rawURL: "vtcp://web.server.prod:443",
			check: func(t *testing.T, ep EndpointSpec) {
				assertEqual(t, "Kind", ep.Kind.String(), "vnet")
				assertEqual(t, "VHostname", ep.VHostname, "web.server")
				assertEqual(t, "Alias", ep.Alias, "prod")
			},
		},
		{
			name:   "socks5 builtin",
			rawURL: "socks5://",
			check: func(t *testing.T, ep EndpointSpec) {
				assertEqual(t, "Kind", ep.Kind.String(), "builtin")
				assertEqual(t, "Scheme", ep.Scheme, "socks5")
			},
		},
		{
			name:   "socks5 with auth",
			rawURL: "socks5://user:pass@0.0.0.0:1080",
			check: func(t *testing.T, ep EndpointSpec) {
				assertEqual(t, "Kind", ep.Kind.String(), "builtin")
				assertEqual(t, "Username", ep.Username, "user")
				assertEqual(t, "Password", ep.Password, "pass")
			},
		},
		{
			name:    "vtcp without dot separator",
			rawURL:  "vtcp://nodot:3306",
			wantErr: true,
		},
		{
			name:    "vtcp with empty alias",
			rawURL:  "vtcp://hostname.:3306",
			wantErr: true,
		},
		{
			name:    "vtcp with empty vhostname",
			rawURL:  "vtcp://.alias:3306",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep, err := normalizeEndpoint(tt.rawURL)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, ep)
		})
	}
}

func TestNormalize_FullConfig(t *testing.T) {
	raw := &RawConfig{
		Links: map[string]string{
			"office": "tcp://relay.corp.com:7000?as=pc-01&auth=secret",
			"home":   "wss://tunnel.home.org/ws?as=macbook&authRef=HOME_AUTH",
		},
		Tunnels: []TunnelRaw{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "access-corp-db",
				Listen: "tcp://127.0.0.1:13306",
				Target: "vtcp://db.office:3306?authcodeRef=DB_AUTH",
			},
			{
				ID:     "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce",
				Name:   "socks-proxy",
				Listen: "vtcp://macbook.home:1080?authcode=socks-key",
				Target: "socks5://",
			},
		},
		Pprof: PprofInfo{Enable: true, Listen: ":6060"},
		API:   APIInfo{Enable: true, Listen: "127.0.0.1:8080"},
	}

	canonical, err := Normalize(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check links
	if len(canonical.Links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(canonical.Links))
	}

	// Check tunnels
	if len(canonical.Tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(canonical.Tunnels))
	}

	// First tunnel
	tunnel0 := canonical.Tunnels[0]
	assertEqual(t, "tunnel0.ID", tunnel0.ID, "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1")
	assertEqual(t, "tunnel0.Listen.Kind", tunnel0.Listen.Kind.String(), "net")
	assertEqual(t, "tunnel0.Target.Kind", tunnel0.Target.Kind.String(), "vnet")
	assertEqual(t, "tunnel0.Target.Alias", tunnel0.Target.Alias, "office")
	assertEqual(t, "tunnel0.Target.AuthcodeRef", tunnel0.Target.AuthcodeRef, "DB_AUTH")

	// Second tunnel
	tunnel1 := canonical.Tunnels[1]
	assertEqual(t, "tunnel1.Listen.Kind", tunnel1.Listen.Kind.String(), "vnet")
	assertEqual(t, "tunnel1.Listen.Authcode", tunnel1.Listen.Authcode, "socks-key")
	assertEqual(t, "tunnel1.Target.Kind", tunnel1.Target.Kind.String(), "builtin")

	// Pprof and API passthrough
	if !canonical.Pprof.Enable {
		t.Error("Pprof should be enabled")
	}
	if !canonical.API.Enable {
		t.Error("API should be enabled")
	}
}

func TestNormalize_NilConfig(t *testing.T) {
	_, err := Normalize(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNormalize_EmptyConfig(t *testing.T) {
	raw := &RawConfig{}
	canonical, err := Normalize(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(canonical.Links) != 0 {
		t.Errorf("expected 0 links, got %d", len(canonical.Links))
	}
	if len(canonical.Tunnels) != 0 {
		t.Errorf("expected 0 tunnels, got %d", len(canonical.Tunnels))
	}
}

func TestParseVtcpHostname(t *testing.T) {
	tests := []struct {
		input     string
		wantHost  string
		wantAlias string
		wantErr   bool
	}{
		{"db.office", "db", "office", false},
		{"web.server.prod", "web.server", "prod", false},
		{"nodot", "", "", true},
		{".alias", "", "", true},
		{"hostname.", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, alias, err := parseVtcpHostname(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, "vhostname", host, tt.wantHost)
			assertEqual(t, "alias", alias, tt.wantAlias)
		})
	}
}

func TestNormalize_UnsupportedOptionsAreExplicit(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		code   string
	}{
		{
			name:   "keepalive value",
			rawURL: "tcp://relay.com:5000?as=node1&keepalive=10",
			code:   ErrKeepaliveNotSupported,
		},
		{
			name:   "keepalive empty value still explicit",
			rawURL: "tcp://relay.com:5000?as=node1&keepalive=",
			code:   ErrKeepaliveNotSupported,
		},
		{
			name:   "via value",
			rawURL: "tcp://relay.com:5000?as=node1&via=gateway.office:7000",
			code:   ErrViaNotSupported,
		},
		{
			name:   "via empty value still explicit",
			rawURL: "tcp://relay.com:5000?as=node1&via=",
			code:   ErrViaNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeLink("home", tt.rawURL)
			if err == nil {
				t.Fatal("expected unsupported option error, got nil")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if ve.Code != tt.code {
				t.Fatalf("code = %s, want %s", ve.Code, tt.code)
			}
			if !strings.Contains(ve.Message, "not supported") {
				t.Fatalf("message should clearly say not supported, got %q", ve.Message)
			}
		})
	}
}

func TestEndpointSpec_Address(t *testing.T) {
	ep := EndpointSpec{Host: "127.0.0.1", Port: "3306"}
	assertEqual(t, "Address", ep.Address(), "127.0.0.1:3306")

	ep2 := EndpointSpec{Host: "localhost"}
	assertEqual(t, "Address no port", ep2.Address(), "localhost")
}

func TestEndpointSpec_IsVNet(t *testing.T) {
	net := EndpointSpec{Kind: EndpointNet}
	vnet := EndpointSpec{Kind: EndpointVNet}
	builtin := EndpointSpec{Kind: EndpointBuiltin}

	if net.IsVNet() {
		t.Error("net endpoint should not be vnet")
	}
	if !vnet.IsVNet() {
		t.Error("vnet endpoint should be vnet")
	}
	if builtin.IsVNet() {
		t.Error("builtin endpoint should not be vnet")
	}
}

func TestEndpointKind_String(t *testing.T) {
	assertEqual(t, "EndpointNet", EndpointNet.String(), "net")
	assertEqual(t, "EndpointVNet", EndpointVNet.String(), "vnet")
	assertEqual(t, "EndpointBuiltin", EndpointBuiltin.String(), "builtin")
	assertEqual(t, "unknown", EndpointKind(99).String(), "unknown")
}

func TestViaSpec_String(t *testing.T) {
	v := ViaSpec{Host: "gateway.office", Port: "7000"}
	assertEqual(t, "ViaSpec.String", v.String(), "gateway.office:7000")
}

func TestNormalize_EndpointAuthcodeFromURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantAC  string
		wantACR string
	}{
		{
			name:   "vtcp with authcode in query",
			rawURL: "vtcp://db.office:3306?authcode=mycode",
			wantAC: "mycode",
		},
		{
			name:    "vtcp with authcodeRef in query",
			rawURL:  "vtcp://db.office:3306?authcodeRef=DB_AUTH_ENV",
			wantACR: "DB_AUTH_ENV",
		},
		{
			name:   "vtcp with both in query",
			rawURL: "vtcp://db.office:3306?authcode=mycode&authcodeRef=DB_AUTH_ENV",
			wantAC: "mycode", wantACR: "DB_AUTH_ENV",
		},
		{
			name:   "tcp with no authcode",
			rawURL: "tcp://127.0.0.1:3306",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep, err := normalizeEndpoint(tt.rawURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, "Authcode", ep.Authcode, tt.wantAC)
			assertEqual(t, "AuthcodeRef", ep.AuthcodeRef, tt.wantACR)
		})
	}
}

func TestNormalize_TunnelAuthcodeFromEndpointURL(t *testing.T) {
	tests := []struct {
		name       string
		raw        TunnelRaw
		wantLsnAC  string
		wantTgtAC  string
		wantTgtACR string
	}{
		{
			name: "authcode on target URL",
			raw: TunnelRaw{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "t1",
				Listen: "tcp://127.0.0.1:3306", Target: "vtcp://db.office:3306?authcode=url-code",
			},
			wantTgtAC: "url-code",
		},
		{
			name: "authcode on listen URL",
			raw: TunnelRaw{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "t1",
				Listen: "vtcp://pc.office:80?authcode=listen-code", Target: "tcp://127.0.0.1:3000",
			},
			wantLsnAC: "listen-code",
		},
		{
			name: "both endpoints have own authcode",
			raw: TunnelRaw{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "t1",
				Listen: "vtcp://a.office:80?authcode=aaa", Target: "vtcp://b.home:3306?authcode=bbb",
			},
			wantLsnAC: "aaa", wantTgtAC: "bbb",
		},
		{
			name: "authcodeRef on target URL",
			raw: TunnelRaw{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "t1",
				Listen: "tcp://127.0.0.1:3306", Target: "vtcp://db.office:3306?authcodeRef=MY_ENV",
			},
			wantTgtACR: "MY_ENV",
		},
		{
			name: "no authcode on either",
			raw: TunnelRaw{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "t1",
				Listen: "tcp://127.0.0.1:3306", Target: "tcp://10.0.0.1:80",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := normalizeTunnel(tt.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, "Listen.Authcode", spec.Listen.Authcode, tt.wantLsnAC)
			assertEqual(t, "Target.Authcode", spec.Target.Authcode, tt.wantTgtAC)
			assertEqual(t, "Target.AuthcodeRef", spec.Target.AuthcodeRef, tt.wantTgtACR)
		})
	}
}

// assertEqual is a test helper for string comparison.
func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

// assertIntEqual is a test helper for int comparison.
func assertIntEqual(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}
