package configv2

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Normalize converts a RawConfig into its canonical internal representation.
// It parses all URLs and extracts structured fields, but does NOT validate
// cross-field constraints (use Validate for that).
func Normalize(raw *RawConfig) (*CanonicalConfig, error) {
	if raw == nil {
		return nil, fmt.Errorf("raw config is nil")
	}

	canonical := &CanonicalConfig{
		Links:   make(map[string]LinkSpec, len(raw.Links)),
		Tunnels: make([]TunnelSpec, 0, len(raw.Tunnels)),
		Pprof:   raw.Pprof,
		API:     raw.API,
	}

	// Normalize links
	for alias, rawURL := range raw.Links {
		spec, err := normalizeLink(alias, rawURL)
		if err != nil {
			return nil, fmt.Errorf("link %q: %w", alias, err)
		}
		canonical.Links[alias] = spec
	}

	// Normalize tunnels
	for i, t := range raw.Tunnels {
		spec, err := normalizeTunnel(t)
		if err != nil {
			return nil, fmt.Errorf("tunnel[%d] %q: %w", i, t.Name, err)
		}
		canonical.Tunnels = append(canonical.Tunnels, spec)
	}

	return canonical, nil
}

// normalizeLink parses a Registration URL into a LinkSpec.
func normalizeLink(alias, rawURL string) (LinkSpec, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return LinkSpec{}, fmt.Errorf("invalid URL: %w", err)
	}

	spec := LinkSpec{
		Alias:  alias,
		Scheme: u.Scheme,
	}

	// Reconstruct relay URL without query params
	relayURL := *u
	relayURL.RawQuery = ""
	relayURL.Fragment = ""
	spec.RelayURL = relayURL.String()

	// Extract query parameters
	q := u.Query()

	spec.As = q.Get("as")
	spec.Auth = q.Get("auth")
	spec.AuthRef = q.Get("authRef")

	// Keepalive: default 15, parse if provided
	spec.Keepalive = 15
	if ka := q.Get("keepalive"); ka != "" {
		val, err := strconv.Atoi(ka)
		if err != nil {
			return LinkSpec{}, fmt.Errorf("invalid keepalive %q: %w", ka, err)
		}
		spec.Keepalive = val
	}

	// Via: optional upstream relay
	if via := q.Get("via"); via != "" {
		viaSpec, err := parseVia(via)
		if err != nil {
			return LinkSpec{}, fmt.Errorf("invalid via %q: %w", via, err)
		}
		spec.Via = viaSpec
	}

	return spec, nil
}

// parseVia parses a via string in the format "host.alias:port".
func parseVia(raw string) (*ViaSpec, error) {
	// Split host:port
	lastColon := strings.LastIndex(raw, ":")
	if lastColon < 0 {
		return nil, fmt.Errorf("must match host.alias:port (port missing)")
	}

	host := raw[:lastColon]
	port := raw[lastColon+1:]

	if host == "" {
		return nil, fmt.Errorf("host part is empty")
	}
	if port == "" {
		return nil, fmt.Errorf("port part is empty")
	}

	// Validate port is numeric
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("port %q is not a valid number", port)
	}

	return &ViaSpec{Host: host, Port: port}, nil
}

// normalizeTunnel converts a TunnelRaw into a TunnelSpec.
func normalizeTunnel(raw TunnelRaw) (TunnelSpec, error) {
	spec := TunnelSpec{
		ID:   raw.ID,
		Name: raw.Name,
	}

	if raw.Listen != "" {
		ep, err := normalizeEndpoint(raw.Listen)
		if err != nil {
			return TunnelSpec{}, fmt.Errorf("listen: %w", err)
		}
		spec.Listen = ep
	}

	if raw.Target != "" {
		ep, err := normalizeEndpoint(raw.Target)
		if err != nil {
			return TunnelSpec{}, fmt.Errorf("target: %w", err)
		}
		spec.Target = ep
	}

	return spec, nil
}

// normalizeEndpoint parses a tunnel endpoint URL into an EndpointSpec.
func normalizeEndpoint(rawURL string) (EndpointSpec, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return EndpointSpec{}, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	spec := EndpointSpec{
		Scheme: u.Scheme,
		RawURL: rawURL,
	}

	// Extract authcode/authcodeRef from query params
	q := u.Query()
	spec.Authcode = q.Get("authcode")
	spec.AuthcodeRef = q.Get("authcodeRef")

	switch u.Scheme {
	case "tcp":
		spec.Kind = EndpointNet
		spec.Host = u.Hostname()
		spec.Port = u.Port()

	case "vtcp":
		spec.Kind = EndpointVNet
		spec.Port = u.Port()
		// Parse vhostname.alias from the hostname part
		hostname := u.Hostname()
		vhostname, alias, err := parseVtcpHostname(hostname)
		if err != nil {
			return EndpointSpec{}, fmt.Errorf("vtcp address %q: %w", hostname, err)
		}
		spec.Host = hostname
		spec.VHostname = vhostname
		spec.Alias = alias

	case "socks5":
		spec.Kind = EndpointBuiltin
		spec.Host = u.Hostname()
		spec.Port = u.Port()
		if u.User != nil {
			spec.Username = u.User.Username()
			spec.Password, _ = u.User.Password()
		}

	default:
		// Allow other schemes to be parsed but mark as unknown kind
		// Validation will reject unsupported schemes
		spec.Kind = EndpointNet
		spec.Host = u.Hostname()
		spec.Port = u.Port()
	}

	return spec, nil
}

// parseVtcpHostname splits "vhostname.alias" into its components.
// The last dot-separated segment is the alias, everything before is the vhostname.
func parseVtcpHostname(hostname string) (vhostname, alias string, err error) {
	lastDot := strings.LastIndex(hostname, ".")
	if lastDot < 0 {
		return "", "", fmt.Errorf("must be vhostname.alias format, got %q", hostname)
	}

	vhostname = hostname[:lastDot]
	alias = hostname[lastDot+1:]

	if vhostname == "" {
		return "", "", fmt.Errorf("vhostname is empty in %q", hostname)
	}
	if alias == "" {
		return "", "", fmt.Errorf("alias is empty in %q", hostname)
	}

	return vhostname, alias, nil
}
