package configv2

import (
	"strings"
	"testing"
)

func TestValidate_ValidFullConfig(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {Alias: "office", Scheme: "tcp", As: "pc-01", Keepalive: 15},
			"home":   {Alias: "home", Scheme: "wss", As: "macbook", Keepalive: 10},
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

func TestValidate_InvalidViaFormat(t *testing.T) {
	cfg := &CanonicalConfig{
		Links: map[string]LinkSpec{
			"office": {
				Alias: "office", Scheme: "tcp", As: "node", Keepalive: 15,
				Via: &ViaSpec{Host: "nodot", Port: "7000"},
			},
		},
	}

	err := Validate(cfg)
	assertValidationError(t, err, ErrInvalidVia)
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
				ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name: "test",
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
						ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
						Name: "test",
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
				ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name: "test",
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
				ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name: "test",
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
				ID:   "c45a8ffb-1a5f-4f78-b9ef-a0e7189bd8c1",
				Name: "test",
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
			"office": {Alias: "office", Scheme: "tcp", As: "pc-01", Keepalive: 15},
			"home":   {Alias: "home", Scheme: "wss", As: "macbook", Keepalive: 10},
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
