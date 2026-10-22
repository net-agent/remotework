package configv2

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Normalize converts a RawConfig into its canonical internal representation.
// It parses all URLs and extracts structured fields, but does NOT validate
// cross-field constraints (use Validate for that).
//
// 遇到当前运行时不支持的选项（via、显式 keepalive）会立即返回带错误码的
// 错误，而不是静默忽略。
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

// normalizeAll 逐项容忍式归一化：坏项跳过并把错误记入 errs，好项保留。
// 与 Normalize 的区别是不因单个坏项拖垮整个配置。
func normalizeAll(raw *RawConfig) (*CanonicalConfig, *ValidationErrors) {
	errs := &ValidationErrors{}
	canonical := &CanonicalConfig{
		Links:   make(map[string]LinkSpec, len(raw.Links)),
		Tunnels: make([]TunnelSpec, 0, len(raw.Tunnels)),
		Pprof:   raw.Pprof,
		API:     raw.API,
	}

	for alias, rawURL := range raw.Links {
		spec, err := normalizeLink(alias, rawURL)
		if err != nil {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs.addLink(alias, ve.Code, ve.Message)
			} else {
				errs.addLink(alias, ErrInvalidItem, err.Error())
			}
			continue
		}
		canonical.Links[alias] = spec
	}

	for i, t := range raw.Tunnels {
		spec, err := normalizeTunnel(t)
		if err != nil {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs.addTunnel(i, t.Name, ve.Code, ve.Message)
			} else {
				errs.addTunnel(i, t.Name, ErrInvalidItem, err.Error())
			}
			continue
		}
		canonical.Tunnels = append(canonical.Tunnels, spec)
	}

	return canonical, errs
}

// normalizeLink parses a Registration URL into a LinkSpec.
func normalizeLink(alias, rawURL string) (LinkSpec, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return LinkSpec{}, fmt.Errorf("invalid URL: %w", err)
	}

	q := u.Query()

	// via/keepalive 在 flex v3.1.0 中没有对应能力，接受它们只会造成静默 no-op。
	// 这里显式拒绝（含空值写法），由 LoadAndCheck 作为坏 link 丢弃。
	if q.Has("via") {
		return LinkSpec{}, &ValidationError{
			Code: ErrViaNotSupported,
			Message: "option 'via' is not supported by flex v3.1.0; " +
				"remove it (upstream relay routing is not implemented)",
		}
	}
	if q.Has("keepalive") {
		return LinkSpec{}, &ValidationError{
			Code: ErrKeepaliveNotSupported,
			Message: "explicit 'keepalive' is not supported by flex v3.1.0; " +
				"remove it (heartbeat interval is controlled by the runtime)",
		}
	}

	spec := LinkSpec{
		Alias:     alias,
		Scheme:    u.Scheme,
		Keepalive: DefaultKeepalive,
	}

	// Reconstruct relay URL without query params, fragment and userinfo.
	// userinfo（wss://user:pass@relay）没有认证语义，保留会经 NetworkReport/API
	// 泄露；这里一律剥离。
	relayURL := *u
	relayURL.User = nil
	relayURL.RawQuery = ""
	relayURL.ForceQuery = false
	relayURL.Fragment = ""
	relayURL.RawFragment = ""
	spec.RelayURL = relayURL.String()

	// Extract query parameters
	spec.As = q.Get("as")
	spec.Auth = q.Get("auth")
	spec.AuthRef = q.Get("authRef")

	return spec, nil
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
//
// authcode/authcodeRef 对所有 scheme 都提取并保留（便于校验与诊断），
// 但只有 vtcp 端点允许使用；物理端点带 authcode 会在 Validate 阶段被拒绝。
func normalizeEndpoint(rawURL string) (EndpointSpec, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return EndpointSpec{}, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	spec := EndpointSpec{
		Scheme: u.Scheme,
		RawURL: rawURL,
	}

	// Extract authcode/authcodeRef from query params for every scheme.
	q := u.Query()
	spec.Authcode = q.Get("authcode")
	spec.AuthcodeRef = q.Get("authcodeRef")

	switch u.Scheme {
	case "tcp", "tcp4", "tcp6":
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
