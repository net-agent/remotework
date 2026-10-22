package configv2

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Predefined error codes matching the design spec.
const (
	ErrTunnelViaNotAllowed   = "ERR_TUNNEL_VIA_NOT_ALLOWED"
	ErrViaNotSupported       = "ERR_VIA_NOT_SUPPORTED"
	ErrKeepaliveNotSupported = "ERR_KEEPALIVE_NOT_SUPPORTED"
	ErrInvalidVia            = "ERR_INVALID_VIA" // Deprecated: via 已不再解析，保留错误码兼容
	ErrVtcpAuthRequired      = "ERR_VTCP_AUTH_REQUIRED"
	ErrAuthcodeNotAllowed    = "ERR_AUTHCODE_NOT_ALLOWED"
	ErrAliasNotFound         = "ERR_ALIAS_NOT_FOUND"
	ErrDuplicateTunnelID     = "ERR_DUPLICATE_TUNNEL_ID"
	ErrDuplicateTunnelName   = "ERR_DUPLICATE_TUNNEL_NAME"
	ErrInvalidAlias          = "ERR_INVALID_ALIAS"
	ErrInvalidTunnelID       = "ERR_INVALID_TUNNEL_ID"
	ErrMissingField          = "ERR_MISSING_FIELD"
	ErrInvalidScheme         = "ERR_INVALID_SCHEME"
	ErrListenConflict        = "ERR_LISTEN_CONFLICT"
	ErrInvalidKeepalive      = "ERR_INVALID_KEEPALIVE"
	ErrMissingAs             = "ERR_MISSING_AS"
	ErrSocks5AuthRequired    = "ERR_SOCKS5_AUTH_REQUIRED"
	ErrAPINonLoopback        = "ERR_API_NON_LOOPBACK"
	ErrPprofNonLoopback      = "ERR_PPROF_NON_LOOPBACK"
	ErrInvalidListen         = "ERR_INVALID_LISTEN"
	ErrConfigParse           = "ERR_CONFIG_PARSE"
	ErrInvalidItem           = "ERR_INVALID_ITEM"
)

// ValidationScope identifies the config section a validation error belongs to.
// LoadAndCheck 使用 Scope/Item/ItemIndex 精确剔除坏项，不依赖错误字符串前缀。
type ValidationScope string

const (
	ScopeConfig ValidationScope = "config"
	ScopeLink   ValidationScope = "link"
	ScopeTunnel ValidationScope = "tunnel"
)

// ValidationError represents a single validation failure with a structured error code.
type ValidationError struct {
	Code    string
	Message string

	Scope     ValidationScope // 错误所属配置区段（config/link/tunnel）
	Item      string          // link alias 或 tunnel name（可能为空）
	ItemIndex int             // 在对应集合中的下标；不适用时为 -1
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ValidationErrors collects multiple validation failures.
type ValidationErrors struct {
	Errors []ValidationError
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("validation failed with %d error(s):\n  %s",
		len(e.Errors), strings.Join(msgs, "\n  "))
}

func (e *ValidationErrors) add(code, msg string) {
	e.addAt(ScopeConfig, "", -1, code, msg)
}

func (e *ValidationErrors) addAt(scope ValidationScope, item string, index int, code, msg string) {
	e.Errors = append(e.Errors, ValidationError{
		Code:      code,
		Message:   msg,
		Scope:     scope,
		Item:      item,
		ItemIndex: index,
	})
}

func (e *ValidationErrors) addLink(alias, code, detail string) {
	msg := detail
	if alias == "" {
		msg = fmt.Sprintf("link with empty alias: %s", detail)
	} else {
		msg = fmt.Sprintf("link %q: %s", alias, detail)
	}
	e.addAt(ScopeLink, alias, -1, code, msg)
}

func (e *ValidationErrors) addTunnel(index int, name, code, detail string) {
	e.addAt(ScopeTunnel, name, index, code,
		fmt.Sprintf("%s: %s", tunnelLabel(index, name), detail))
}

func (e *ValidationErrors) hasErrors() bool {
	return len(e.Errors) > 0
}

func tunnelLabel(index int, name string) string {
	if name == "" {
		return fmt.Sprintf("tunnel[%d]", index)
	}
	return fmt.Sprintf("tunnel[%d] %q", index, name)
}

// aliasPattern allows only [a-zA-Z0-9-] for link aliases.
var aliasPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// uuidPattern validates UUID v4 format (loose: any hex UUID with dashes).
var uuidPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

// forbiddenTunnelSchemes are not allowed as tunnel endpoints.
var forbiddenTunnelSchemes = map[string]bool{
	"http":  true,
	"https": true,
	"ws":    true,
	"wss":   true,
}

// tunnelSchemeKinds maps a supported tunnel endpoint scheme to its endpoint kind.
var tunnelSchemeKinds = map[string]EndpointKind{
	"tcp":    EndpointNet,
	"tcp4":   EndpointNet,
	"tcp6":   EndpointNet,
	"vtcp":   EndpointVNet,
	"socks5": EndpointBuiltin,
}

// supportedLinkSchemes 是 flex buildConnector 实际支持的注册 URL scheme。
// 空 scheme 被 buildConnector 当作 tcp，保留兼容。
var supportedLinkSchemes = map[string]bool{
	"":    true,
	"tcp": true,
	"ws":  true,
	"wss": true,
}

// Validate checks all cross-field constraints on a CanonicalConfig.
// Returns nil if valid, or a *ValidationErrors with all found issues.
//
// 校验是逐项独立的：返回的 ValidationError 带 Scope/Item/ItemIndex，
// LoadAndCheck 据此只丢弃真正有错的 link/tunnel。跨项冲突（重复 id/name/listen）
// 只会在通过单项校验的条目之间检测，避免“坏项拖累好项”。
func Validate(cfg *CanonicalConfig) error {
	errs := &ValidationErrors{}
	if cfg == nil {
		errs.add(ErrConfigParse, "canonical config is nil")
		return errs
	}

	validLinks := validateLinks(cfg, errs)
	validateTunnels(cfg, validLinks, errs)
	validateServerListens(cfg, errs)

	if errs.hasErrors() {
		return errs
	}
	return nil
}

// validateLinks checks link-level requirements and returns the set of usable
// aliases. A link with any error is unusable, including for vtcp endpoints that
// reference it.
func validateLinks(cfg *CanonicalConfig, errs *ValidationErrors) map[string]bool {
	valid := make(map[string]bool, len(cfg.Links))

	aliases := make([]string, 0, len(cfg.Links))
	for alias := range cfg.Links {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	for _, alias := range aliases {
		link := cfg.Links[alias]
		ok := true
		add := func(code, detail string) {
			errs.addLink(alias, code, detail)
			ok = false
		}

		// Rule 1: alias must be unique (map guarantees this) and valid format
		if !aliasPattern.MatchString(alias) {
			add(ErrInvalidAlias, "alias must match [a-zA-Z0-9-]")
		}

		// Registration URL: as is MUST
		if link.As == "" {
			add(ErrMissingAs, "'as' parameter is required")
		}

		// Link scheme is consumed by vnet.buildConnector at runtime.
		if !supportedLinkSchemes[strings.ToLower(link.Scheme)] {
			add(ErrInvalidScheme,
				fmt.Sprintf("link scheme %q is not supported (supported: tcp, ws, wss)", link.Scheme))
		}

		// Rule 12: keepalive must be positive; any explicit non-default value
		// is an unsupported no-op (flex v3.1.0 controls heartbeat internally).
		switch {
		case link.Keepalive < 1:
			add(ErrInvalidKeepalive,
				fmt.Sprintf("keepalive must be positive, got %d", link.Keepalive))
		case link.Keepalive != DefaultKeepalive:
			add(ErrKeepaliveNotSupported,
				fmt.Sprintf("explicit keepalive %d is not supported by flex v3.1.0; "+
					"use the runtime default %d", link.Keepalive, DefaultKeepalive))
		}

		// Rule 9: via is not implemented by flex v3.1.0.
		if link.Via != nil {
			add(ErrViaNotSupported,
				"option 'via' is not supported by flex v3.1.0; remove it")
		}

		if ok {
			valid[alias] = true
		}
	}

	return valid
}

// validateTunnels checks rules 2-8, 10-11, 13 and prunes per item.
func validateTunnels(cfg *CanonicalConfig, validLinks map[string]bool, errs *ValidationErrors) {
	intrinsicOK := make([]bool, len(cfg.Tunnels))

	for i, t := range cfg.Tunnels {
		ok := true
		add := func(code, detail string) {
			errs.addTunnel(i, t.Name, code, detail)
			ok = false
		}

		// Rule 2: id must be UUID
		if t.ID == "" {
			add(ErrInvalidTunnelID, "id is required")
		} else if !uuidPattern.MatchString(t.ID) {
			add(ErrInvalidTunnelID, fmt.Sprintf("id %q is not a valid UUID", t.ID))
		}

		// Rule 3: name must exist
		if t.Name == "" {
			add(ErrMissingField, "name is required")
		}

		// Rule 4: listen and target must exist
		if t.Listen.RawURL == "" {
			add(ErrMissingField, "listen is required")
		}
		if t.Target.RawURL == "" {
			add(ErrMissingField, "target is required")
		}

		// Validate listen endpoint
		if t.Listen.RawURL != "" {
			validateTunnelEndpoint(validLinks, "listen", t.Listen, add)
		}

		// Validate target endpoint
		if t.Target.RawURL != "" {
			validateTunnelEndpoint(validLinks, "target", t.Target, add)
		}

		// Rule 10-11: each vtcp endpoint must carry authcode or authcodeRef
		// is handled inside validateTunnelEndpoint.

		// Rule 14: open socks5 proxy protection. Credentials for the built-in
		// socks5 service live on the target endpoint URL (socks5://user:pass@).
		validateSocks5Credentials(t, add)

		intrinsicOK[i] = ok
	}

	// Cross-item checks only consider intrinsically valid tunnels. Processed in
	// config order, first occurrence wins; a loser is dropped and does not
	// register its id/name/listen, so it cannot drag down later valid entries.
	idSet := make(map[string]string)
	nameSet := make(map[string]string)
	listenSet := make(map[string]string)

	for i, t := range cfg.Tunnels {
		if !intrinsicOK[i] {
			continue
		}
		label := tunnelLabel(i, t.Name)

		if owner, dup := idSet[t.ID]; dup {
			errs.addTunnel(i, t.Name, ErrDuplicateTunnelID,
				fmt.Sprintf("tunnel id %q must be unique (already used by %s)", t.ID, owner))
			continue
		}
		if owner, dup := nameSet[t.Name]; dup {
			errs.addTunnel(i, t.Name, ErrDuplicateTunnelName,
				fmt.Sprintf("tunnel name %q must be unique (kernel key tunnel/<name>; already used by %s)",
					t.Name, owner))
			continue
		}

		key := listenKey(t.Listen)
		if owner, dup := listenSet[key]; dup {
			errs.addTunnel(i, t.Name, ErrListenConflict,
				fmt.Sprintf("listen endpoint %s conflicts with %s (same listen address)", key, owner))
			continue
		}

		idSet[t.ID] = label
		nameSet[t.Name] = label
		listenSet[key] = label
	}
}

// validateTunnelEndpoint checks rules 5-8 and authcode rules for one endpoint.
func validateTunnelEndpoint(
	validLinks map[string]bool,
	role string,
	ep EndpointSpec,
	add func(code, detail string),
) {
	// Rule 7: tunnel endpoint must not carry via
	if hasQueryParam(ep.RawURL, "via") {
		add(ErrTunnelViaNotAllowed, "via is only allowed in registration URL")
	}

	// Physical/builtin endpoints cannot use transport encryption: accepting
	// authcode there would make users believe traffic is encrypted while it is
	// actually sent in plaintext. Check this before scheme/shape checks so the
	// security-relevant error is reported even for an otherwise unknown scheme.
	if ep.Kind != EndpointVNet && (ep.Authcode != "" || ep.AuthcodeRef != "") {
		add(ErrAuthcodeNotAllowed,
			"authcode/authcodeRef is only supported on vtcp endpoints; "+
				"physical endpoints are not encrypted")
	}

	// Rule 8: tunnel endpoint must use a supported scheme.
	scheme := strings.ToLower(ep.Scheme)
	expectedKind, known := tunnelSchemeKinds[scheme]
	if !known {
		if forbiddenTunnelSchemes[scheme] {
			add(ErrInvalidScheme,
				fmt.Sprintf("scheme %q is not allowed as tunnel endpoint", ep.Scheme))
		} else {
			add(ErrInvalidScheme,
				fmt.Sprintf("unknown tunnel endpoint scheme %q (supported: tcp, tcp4, tcp6, vtcp, socks5)", ep.Scheme))
		}
		return
	}
	if expectedKind != ep.Kind {
		add(ErrInvalidScheme,
			fmt.Sprintf("scheme %q is inconsistent with endpoint kind %s", ep.Scheme, ep.Kind))
		return
	}

	if ep.Kind == EndpointBuiltin {
		// socks5 is a built-in service that can only be a target.
		if role == "listen" {
			add(ErrInvalidScheme, "socks5 endpoint is only allowed as target")
		}
		return
	}

	if ep.Host == "" {
		add(ErrMissingField, "host is required")
	}
	if ep.Port == "" {
		add(ErrMissingField, "port is required")
	}

	if ep.Kind != EndpointVNet {
		return
	}

	// Rule 10-11: each vtcp endpoint must carry authcode or authcodeRef.
	if ep.Authcode == "" && ep.AuthcodeRef == "" {
		add(ErrVtcpAuthRequired, "authcode or authcodeRef is required for vtcp endpoint")
	}

	// Rule 6: vtcp alias must exist in usable links.
	if !validLinks[ep.Alias] {
		add(ErrAliasNotFound, fmt.Sprintf("alias %q not found in usable links", ep.Alias))
	}
}

// validateSocks5Credentials rejects an unauthenticated SOCKS5 proxy exposed on
// a non-loopback physical listener. The credentials are configured on the
// built-in target URL: socks5://user:pass@.
func validateSocks5Credentials(t TunnelSpec, add func(code, detail string)) {
	if t.Listen.RawURL == "" || t.Target.Kind != EndpointBuiltin ||
		strings.ToLower(t.Target.Scheme) != "socks5" {
		return
	}
	if t.Listen.Kind != EndpointNet || isLoopbackHost(t.Listen.Host) {
		return
	}
	if t.Target.Username == "" || t.Target.Password == "" {
		add(ErrSocks5AuthRequired,
			"non-loopback physical listen requires socks5 username and password "+
				"(otherwise it is an open proxy)")
	}
}

// validateServerListens enforces that optional control/debug listeners do not
// accidentally bind non-loopback addresses.
func validateServerListens(cfg *CanonicalConfig, errs *ValidationErrors) {
	if cfg.Pprof.Enable && cfg.Pprof.Listen == "" {
		errs.add(ErrMissingField, "pprof.listen is required when pprof is enabled")
	}
	if cfg.Pprof.Listen != "" {
		host, err := listenHost(cfg.Pprof.Listen)
		if err != nil {
			errs.add(ErrInvalidListen,
				fmt.Sprintf("pprof.listen %q is invalid: %v", cfg.Pprof.Listen, err))
		} else if host != "" && !isLoopbackHost(host) {
			// 空 host（如 ":6060"）由运行时强制重映射到 127.0.0.1；
			// 显式指定 0.0.0.0/:: 等非回环地址仍然拒绝。
			errs.add(ErrPprofNonLoopback,
				fmt.Sprintf("pprof.listen %q must bind a loopback address", cfg.Pprof.Listen))
		}
	}

	if cfg.API.Listen != "" {
		host, err := listenHost(cfg.API.Listen)
		if err != nil {
			errs.add(ErrInvalidListen,
				fmt.Sprintf("api.listen %q is invalid: %v", cfg.API.Listen, err))
		} else if !isLoopbackHost(host) && !cfg.API.AllowNonLoopback {
			errs.add(ErrAPINonLoopback,
				fmt.Sprintf("api.listen %q is not loopback; set api.allowNonLoopback=true "+
					"and configure an API token to allow remote access", cfg.API.Listen))
		}
	}
}

// hasQueryParam checks if the raw URL carries the given query parameter
// (including an empty value).
func hasQueryParam(rawURL, key string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Query().Has(key)
}

// listenHost extracts the host part of a host:port listen address.
func listenHost(listen string) (string, error) {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return "", err
	}
	return host, nil
}

// isLoopbackHost reports whether host is a literal loopback address or
// "localhost". Hostnames are not resolved: anything not provably loopback is
// treated as non-loopback.
func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// listenKey returns the conflict-detection key for a listen endpoint.
func listenKey(ep EndpointSpec) string {
	if ep.Kind == EndpointVNet {
		return fmt.Sprintf("%s://%s/%s", strings.ToLower(ep.Scheme), ep.Alias, ep.Address())
	}
	return fmt.Sprintf("%s://%s", strings.ToLower(ep.Scheme), ep.Address())
}
