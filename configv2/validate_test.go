package configv2

import (
	"strings"
	"testing"
)

func TestValidate_ValidFullConfig(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "pc-01", Keepalive: DefaultKeepalive},
			"home":   {Alias: "home", Scheme: "wss", As: "macbook", Keepalive: DefaultKeepalive},
		},
		Tunnels: []TunnelSpec{
			{
				ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name: "net-to-vnet",
				Listen: EndpointSpec{
					Kind: EndpointNet, Scheme: "tcp",
					Host: "127.0.0.1", Port: "13306",
					RawURL: "tcp://127.0.0.1:13306",
				},
				Target: EndpointSpec{
					Kind: EndpointVNet, Scheme: "vtcp",
					Host: "db.office", Port: "3306",
					VHostname: "db", Alias: "office",
					AuthcodeRef: "DB_AUTH",
					RawURL:      "vtcp://db.office:3306?authcodeRef=DB_AUTH",
				},
			},
			{
				ID:   "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce",
				Name: "net-to-net",
				Listen: EndpointSpec{
					Kind: EndpointNet, Scheme: "tcp",
					Host: "127.0.0.1", Port: "8080",
					RawURL: "tcp://127.0.0.1:8080",
				},
				Target: EndpointSpec{
					Kind: EndpointNet, Scheme: "tcp",
					Host: "10.0.0.12", Port: "80",
					RawURL: "tcp://10.0.0.12:80",
				},
			},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidAlias(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"invalid alias!": {Alias: "invalid alias!", Scheme: "tcp", As: "node", Keepalive: 15},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrInvalidAlias)
}

func TestValidate_MissingAs(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "", Keepalive: 15},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrMissingAs)
}

func TestValidate_InvalidKeepalive(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "node", Keepalive: 0},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrInvalidKeepalive)
}

func TestValidate_ViaUnsupported(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {
				Alias: "office", Scheme: "tcp", As: "node", Keepalive: DefaultKeepalive,
				Via: &ViaSpec{Host: "gateway.office", Port: "7000"},
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrViaNotSupported)
}

func TestValidate_ExplicitKeepaliveUnsupported(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "node", Keepalive: 10},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrKeepaliveNotSupported)
}

func TestValidate_UnsupportedLinkScheme(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "udp", As: "node", Keepalive: DefaultKeepalive},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrInvalidScheme)
}

func TestValidate_TunnelMissingID(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{Name: "test", Listen: tcpEndpoint("127.0.0.1", "80"), Target: tcpEndpoint("10.0.0.1", "80")},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrInvalidTunnelID)
}

func TestValidate_TunnelInvalidUUID(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{
				ID: "not-a-uuid", Name: "test",
				Listen: tcpEndpoint("127.0.0.1", "80"), Target: tcpEndpoint("10.0.0.1", "80"),
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrInvalidTunnelID)
}

func TestValidate_DuplicateTunnelID(t *testing.T) {
	id := "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{ID: id, Name: "rule-a", Listen: tcpEndpoint("127.0.0.1", "80"), Target: tcpEndpoint("10.0.0.1", "80")},
			{ID: id, Name: "rule-b", Listen: tcpEndpoint("127.0.0.1", "81"), Target: tcpEndpoint("10.0.0.1", "81")},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrDuplicateTunnelID)
}

func TestValidate_TunnelMissingName(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "", // missing name
				Listen: tcpEndpoint("127.0.0.1", "80"),
				Target: tcpEndpoint("10.0.0.1", "80"),
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrMissingField)
}

func TestValidate_TunnelMissingListen(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "test",
				Target: tcpEndpoint("10.0.0.1", "80"),
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrMissingField)
}

func TestValidate_TunnelMissingTarget(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "test",
				Listen: tcpEndpoint("127.0.0.1", "80"),
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrMissingField)
}

func TestValidate_TunnelViaNotAllowed(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "pc-01", Keepalive: 15},
		},
		Tunnels: []TunnelSpec{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "test",
				Listen: tcpEndpoint("127.0.0.1", "13306"),
				Target: EndpointSpec{
					Kind: EndpointVNet, Scheme: "vtcp",
					Host: "db.office", Port: "3306",
					VHostname: "db", Alias: "office",
					AuthcodeRef: "AUTH",
					RawURL:      "vtcp://db.office:3306?via=gateway.home:7000&authcodeRef=AUTH",
				},
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrTunnelViaNotAllowed)
}

func TestValidate_ForbiddenScheme(t *testing.T) {
	schemes := []string{"http", "https", "ws", "wss"}
	for _, scheme := range schemes {
		t.Run(scheme, func(t *testing.T) {
			cfg := &CanonicalConfig{
				Links: map[string]LinkSpec{},
				Tunnels: []TunnelSpec{
					{
						ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
						Name:   "test",
						Listen: tcpEndpoint("127.0.0.1", "80"),
						Target: EndpointSpec{
							Kind: EndpointNet, Scheme: scheme,
							Host: "example.com", Port: "80",
							RawURL: scheme + "://example.com:80",
						},
					},
				},
			}

			err := Validate(cfg)
			assertValidationError(t, err, ErrInvalidScheme)
		})
	}
}

func TestValidate_VtcpAliasNotFound(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{}, // no links defined
		Tunnels: []TunnelSpec{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "test",
				Listen: tcpEndpoint("127.0.0.1", "13306"),
				Target: EndpointSpec{
					Kind: EndpointVNet, Scheme: "vtcp",
					Host: "db.office", Port: "3306",
					VHostname: "db", Alias: "office",
					AuthcodeRef: "AUTH",
					RawURL:      "vtcp://db.office:3306?authcodeRef=AUTH",
				},
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrAliasNotFound)
}

func TestValidate_VtcpAuthRequired(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "pc-01", Keepalive: 15},
		},
		Tunnels: []TunnelSpec{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "test",
				Listen: tcpEndpoint("127.0.0.1", "13306"),
				Target: EndpointSpec{
					Kind: EndpointVNet, Scheme: "vtcp",
					Host: "db.office", Port: "3306",
					VHostname: "db", Alias: "office",
					RawURL: "vtcp://db.office:3306",
				},
				// No authcode or authcodeRef!
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrVtcpAuthRequired)
}

func TestValidate_VtcpAuthWithAuthcode(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "pc-01", Keepalive: 15},
		},
		Tunnels: []TunnelSpec{
			{
				ID:     "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name:   "test",
				Listen: tcpEndpoint("127.0.0.1", "13306"),
				Target: EndpointSpec{
					Kind: EndpointVNet, Scheme: "vtcp",
					Host: "db.office", Port: "3306",
					VHostname: "db", Alias: "office",
					Authcode: "plain-code",
					RawURL:   "vtcp://db.office:3306?authcode=plain-code",
				},
			},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected valid config with authcode, got error: %v", err)
	}
}

func TestValidate_ListenConflict(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "rule-a",
				Listen: tcpEndpoint("127.0.0.1", "8080"),
				Target: tcpEndpoint("10.0.0.1", "80"),
			},
			{
				ID: "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce", Name: "rule-b",
				Listen: tcpEndpoint("127.0.0.1", "8080"), // same listen!
				Target: tcpEndpoint("10.0.0.2", "80"),
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrListenConflict)

	// 冲突消息必须指明冲突双方，便于定位
	verrs := err.(*ValidationErrors)
	var conflictMsg string
	for _, e := range verrs.Errors {
		if e.Code == ErrListenConflict {
			conflictMsg = e.Message
		}
	}
	if !strings.Contains(conflictMsg, `"rule-a"`) || !strings.Contains(conflictMsg, `"rule-b"`) {
		t.Errorf("conflict message should name both tunnels, got: %s", conflictMsg)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"bad alias!": {Alias: "bad alias!", Scheme: "tcp", As: "", Keepalive: 0},
		},
		Tunnels: []TunnelSpec{
			{Name: ""}, // missing everything
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected errors, got nil")
	}

	verrs, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}

	// Should have multiple errors
	if len(verrs.Errors) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(verrs.Errors), verrs)
	}
}

func TestValidate_EmptyConfig(t *testing.T) {
	cfg := &CanonicalConfig{
		Links:   map[string]LinkSpec{},
		Tunnels: []TunnelSpec{},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("empty config should be valid, got error: %v", err)
	}
}

func TestValidate_NetToNetNoAuth(t *testing.T) {
	// net -> net tunnels should NOT require authcode
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{},
		Tunnels: []TunnelSpec{
			{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "local-relay",
				Listen: tcpEndpoint("127.0.0.1", "8080"),
				Target: tcpEndpoint("10.0.0.12", "80"),
				// No authcode needed - no vtcp involved
			},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("net-to-net tunnel should be valid without authcode, got error: %v", err)
	}
}

func TestValidate_VnetToVnetRequiresAuth(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "pc-01", Keepalive: DefaultKeepalive},
			"home":   {Alias: "home", Scheme: "wss", As: "macbook", Keepalive: DefaultKeepalive},
		},
		Tunnels: []TunnelSpec{
			{
				ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name: "cross-vnet",
				Listen: EndpointSpec{
					Kind: EndpointVNet, Scheme: "vtcp",
					Host: "a.office", Port: "9000",
					VHostname: "a", Alias: "office",
					RawURL: "vtcp://a.office:9000",
					// Missing authcode!
				},
				Target: EndpointSpec{
					Kind: EndpointVNet, Scheme: "vtcp",
					Host: "b.home", Port: "9000",
					VHostname: "b", Alias: "home",
					RawURL: "vtcp://b.home:9000",
					// Missing authcode!
				},
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrVtcpAuthRequired)
}

func TestValidate_PhysicalEndpointAuthcodeRejected(t *testing.T) {
	tests := []struct {
		name string
		spec TunnelSpec
	}{
		{
			name: "listen authcode",
			spec: TunnelSpec{
				ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name: "listen-authcode",
				Listen: EndpointSpec{
					Kind: EndpointNet, Scheme: "tcp",
					Host: "127.0.0.1", Port: "8080",
					Authcode: "super-secret",
					RawURL:   "tcp://127.0.0.1:8080?authcode=super-secret",
				},
				Target: tcpEndpoint("10.0.0.1", "80"),
			},
		},
		{
			name: "target authcodeRef",
			spec: TunnelSpec{
				ID:     "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce",
				Name:   "target-authcode-ref",
				Listen: tcpEndpoint("127.0.0.1", "8080"),
				Target: EndpointSpec{
					Kind: EndpointNet, Scheme: "tcp",
					Host: "10.0.0.1", Port: "80",
					AuthcodeRef: "PHYSICAL_SECRET",
					RawURL:      "tcp://10.0.0.1:80?authcodeRef=PHYSICAL_SECRET",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&CanonicalConfig{Tunnels: []TunnelSpec{tt.spec}})
			assertValidationError(t, err, ErrAuthcodeNotAllowed)
			if strings.Contains(err.Error(), "super-secret") {
				t.Errorf("validation error leaked authcode: %v", err)
			}
		})
	}
}

func TestValidate_TunnelDuplicateName(t *testing.T) {
	cfg := &CanonicalConfig{
		Tunnels: []TunnelSpec{
			{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "same-name",
				Listen: tcpEndpoint("127.0.0.1", "8080"),
				Target: tcpEndpoint("10.0.0.1", "80"),
			},
			{
				ID: "f56d9858-5d57-4a35-a7b7-3e683fe2a8ce", Name: "same-name",
				Listen: tcpEndpoint("127.0.0.1", "8081"),
				Target: tcpEndpoint("10.0.0.1", "81"),
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrDuplicateTunnelName)

	verrs := err.(*ValidationErrors)
	if len(verrs.Errors) != 1 {
		t.Fatalf("expected only the second tunnel to be rejected, got %v", verrs)
	}
	if verrs.Errors[0].ItemIndex != 1 || verrs.Errors[0].Item != "same-name" {
		t.Fatalf("duplicate name should point at index 1, got %+v", verrs.Errors[0])
	}
}

func TestValidate_UnknownTunnelScheme(t *testing.T) {
	cfg := &CanonicalConfig{
		Tunnels: []TunnelSpec{
			{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "udp",
				Listen: tcpEndpoint("127.0.0.1", "8080"),
				Target: EndpointSpec{
					Kind: EndpointNet, Scheme: "udp",
					Host: "10.0.0.1", Port: "53",
					RawURL: "udp://10.0.0.1:53",
				},
			},
		},
	}
	assertValidationError(t, Validate(cfg), ErrInvalidScheme)

	socksListen := &CanonicalConfig{
		Tunnels: []TunnelSpec{
			{
				ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "socks-listen",
				Listen: EndpointSpec{
					Kind: EndpointBuiltin, Scheme: "socks5", RawURL: "socks5://",
				},
				Target: tcpEndpoint("10.0.0.1", "80"),
			},
		},
	}
	assertValidationError(t, Validate(socksListen), ErrInvalidScheme)
}

func TestValidate_EndpointMissingHostPort(t *testing.T) {
	tests := []struct {
		name   string
		listen EndpointSpec
		target EndpointSpec
	}{
		{
			name:   "missing listen host",
			listen: EndpointSpec{Kind: EndpointNet, Scheme: "tcp", Host: "", Port: "8080", RawURL: "tcp://:8080"},
			target: tcpEndpoint("10.0.0.1", "80"),
		},
		{
			name:   "missing target port",
			listen: tcpEndpoint("127.0.0.1", "8080"),
			target: EndpointSpec{Kind: EndpointNet, Scheme: "tcp", Host: "10.0.0.1", RawURL: "tcp://10.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &CanonicalConfig{
				Tunnels: []TunnelSpec{{
					ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "missing",
					Listen: tt.listen, Target: tt.target,
				}},
			}
			assertValidationError(t, Validate(cfg), ErrMissingField)
		})
	}
}

func TestValidate_Socks5OpenProxyPolicy(t *testing.T) {
	socksTarget := func(user, pass string) EndpointSpec {
		return EndpointSpec{
			Kind: EndpointBuiltin, Scheme: "socks5",
			Username: user, Password: pass,
			RawURL: "socks5://",
		}
	}

	tests := []struct {
		name    string
		listen  EndpointSpec
		target  EndpointSpec
		links   map[string]LinkSpec
		wantErr bool
	}{
		{
			name:    "non-loopback without credentials rejected",
			listen:  tcpEndpoint("0.0.0.0", "1080"),
			target:  socksTarget("", ""),
			wantErr: true,
		},
		{
			name:   "non-loopback with credentials allowed",
			listen: tcpEndpoint("0.0.0.0", "1080"),
			target: socksTarget("user", "pass"),
		},
		{
			name:   "loopback without credentials allowed",
			listen: tcpEndpoint("127.0.0.1", "1080"),
			target: socksTarget("", ""),
		},
		{
			name: "vnet listen without credentials allowed",
			listen: EndpointSpec{
				Kind: EndpointVNet, Scheme: "vtcp",
				Host: "pc.office", Port: "1080",
				VHostname: "pc", Alias: "office",
				Authcode: "code",
				RawURL:   "vtcp://pc.office:1080?authcode=code",
			},
			target: socksTarget("", ""),
			links: map[string]LinkSpec{
				"office": {Alias: "office", Scheme: "tcp", As: "node", Keepalive: DefaultKeepalive},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &CanonicalConfig{
				Links: tt.links,
				Tunnels: []TunnelSpec{{
					ID: "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1", Name: "socks",
					Listen: tt.listen, Target: tt.target,
				}},
			}
			err := Validate(cfg)
			if tt.wantErr {
				assertValidationError(t, err, ErrSocks5AuthRequired)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidate_ServerListenPolicy(t *testing.T) {
	validLink := map[string]LinkSpec{}

	tests := []struct {
		name    string
		cfg     CanonicalConfig
		wantErr string
	}{
		{
			name:    "pprof non-loopback rejected",
			cfg:     CanonicalConfig{Links: validLink, Pprof: PprofInfo{Enable: true, Listen: "0.0.0.0:6060"}},
			wantErr: ErrPprofNonLoopback,
		},
		{
			name: "pprof localhost allowed",
			cfg:  CanonicalConfig{Links: validLink, Pprof: PprofInfo{Enable: true, Listen: "localhost:6060"}},
		},
		{
			name:    "pprof invalid listen",
			cfg:     CanonicalConfig{Links: validLink, Pprof: PprofInfo{Enable: true, Listen: "6060"}},
			wantErr: ErrInvalidListen,
		},
		{
			name:    "pprof enabled without listen",
			cfg:     CanonicalConfig{Links: validLink, Pprof: PprofInfo{Enable: true}},
			wantErr: ErrMissingField,
		},
		{
			name:    "api non-loopback rejected by default",
			cfg:     CanonicalConfig{Links: validLink, API: APIInfo{Enable: true, Listen: "0.0.0.0:8080"}},
			wantErr: ErrAPINonLoopback,
		},
		{
			name: "api non-loopback explicitly allowed",
			cfg: CanonicalConfig{Links: validLink, API: APIInfo{
				Enable: true, Listen: "0.0.0.0:8080", AllowNonLoopback: true,
			}},
		},
		{
			name: "api loopback allowed",
			cfg:  CanonicalConfig{Links: validLink, API: APIInfo{Enable: true, Listen: "127.0.0.1:8080"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func TestValidate_ErrorCarriesScopeAndItem(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "", Keepalive: DefaultKeepalive},
		},
		Tunnels: []TunnelSpec{
			{Name: "broken"},
		},
		Pprof: PprofInfo{Enable: true, Listen: "0.0.0.0:6060"},
	}

	err := Validate(cfg)
	verrs, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T", err)
	}

	var linkErr, tunnelErr, configErr *ValidationError
	for i := range verrs.Errors {
		e := &verrs.Errors[i]
		switch e.Scope {
		case ScopeLink:
			linkErr = e
		case ScopeTunnel:
			tunnelErr = e
		case ScopeConfig:
			configErr = e
		}
	}
	if linkErr == nil || linkErr.Item != "office" || linkErr.ItemIndex != -1 {
		t.Fatalf("bad link scope fields: %+v", linkErr)
	}
	if tunnelErr == nil || tunnelErr.Item != "broken" || tunnelErr.ItemIndex != 0 {
		t.Fatalf("bad tunnel scope fields: %+v", tunnelErr)
	}
	if configErr == nil || configErr.Item != "" || configErr.ItemIndex != -1 {
		t.Fatalf("bad config scope fields: %+v", configErr)
	}
}

func TestValidate_CrossChecksIgnoreIntrinsicBadItems(t *testing.T) {
	// 坏项（缺 name / 缺 id）会整条被丢弃，不应把同 listen/同 id 的好项
	// 误判为冲突并连带丢弃。
	badID := "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1"
	cfg := &CanonicalConfig{
		Tunnels: []TunnelSpec{
			{
				ID:     badID,
				Name:   "", // intrinsic error: missing name
				Listen: tcpEndpoint("127.0.0.1", "8080"),
				Target: tcpEndpoint("10.0.0.1", "80"),
			},
			{
				ID:     badID, // same id, but first tunnel is dropped
				Name:   "good",
				Listen: tcpEndpoint("127.0.0.1", "8080"), // same listen too
				Target: tcpEndpoint("10.0.0.2", "80"),
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected intrinsic error for the bad tunnel")
	}
	verrs := err.(*ValidationErrors)
	for _, e := range verrs.Errors {
		if e.Code == ErrDuplicateTunnelID || e.Code == ErrListenConflict {
			t.Fatalf("good tunnel wrongly flagged as conflict: %v", e)
		}
	}
	for _, e := range verrs.Errors {
		if e.ItemIndex != 0 {
			t.Fatalf("all errors should belong to the bad tunnel at index 0, got %+v", e)
		}
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Code: "TEST_CODE", Message: "test message"}
	assertEqual(t, "Error", err.Error(), "TEST_CODE: test message")
}

func TestValidationErrors_Error(t *testing.T) {
	errs := &ValidationErrors{}
	assertEqual(t, "empty", errs.Error(), "no validation errors")

	errs.add("CODE1", "msg1")
	errs.add("CODE2", "msg2")
	got := errs.Error()
	if !strings.Contains(got, "2 error(s)") {
		t.Errorf("expected '2 error(s)' in %q", got)
	}
}

// Helper to create a simple tcp endpoint for tests.
func tcpEndpoint(host, port string) EndpointSpec {
	return EndpointSpec{
		Kind:   EndpointNet,
		Scheme: "tcp",
		Host:   host,
		Port:   port,
		RawURL: "tcp://" + host + ":" + port,
	}
}

// assertValidationError checks that the error contains a specific error code.
func assertValidationError(t *testing.T, err error, expectedCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", expectedCode)
	}
	verrs, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected *ValidationErrors, got %T: %v", err, err)
	}
	for _, e := range verrs.Errors {
		if e.Code == expectedCode {
			return
		}
	}
	t.Errorf("expected error code %s, got errors: %v", expectedCode, verrs)
}
