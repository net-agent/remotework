package configv2

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Predefined error codes matching the design spec.
const (
	ErrTunnelViaNotAllowed = "ERR_TUNNEL_VIA_NOT_ALLOWED"
	ErrInvalidVia          = "ERR_INVALID_VIA"
	ErrVtcpAuthRequired    = "ERR_VTCP_AUTH_REQUIRED"
	ErrAliasNotFound       = "ERR_ALIAS_NOT_FOUND"
	ErrDuplicateTunnelID   = "ERR_DUPLICATE_TUNNEL_ID"
	ErrInvalidAlias        = "ERR_INVALID_ALIAS"
	ErrInvalidTunnelID     = "ERR_INVALID_TUNNEL_ID"
	ErrMissingField        = "ERR_MISSING_FIELD"
	ErrInvalidScheme       = "ERR_INVALID_SCHEME"
	ErrListenConflict      = "ERR_LISTEN_CONFLICT"
	ErrInvalidKeepalive    = "ERR_INVALID_KEEPALIVE"
	ErrMissingAs           = "ERR_MISSING_AS"
)

// ValidationError represents a single validation failure with a structured error code.
type ValidationError struct {
	Code    string
	Message string
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
	e.Errors = append(e.Errors, ValidationError{Code: code, Message: msg})
}

func (e *ValidationErrors) hasErrors() bool {
	return len(e.Errors) > 0
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

// Validate checks all cross-field constraints on a CanonicalConfig.
// Returns nil if valid, or a *ValidationErrors with all found issues.
func Validate(cfg *CanonicalConfig) error {
	errs := &ValidationErrors{}

	validateLinks(cfg, errs)
	validateTunnels(cfg, errs)

	if errs.hasErrors() {
		return errs
	}
	return nil
}

// validateLinks checks rules 1, 9, 12, and link-level requirements.
func validateLinks(cfg *CanonicalConfig, errs *ValidationErrors) {
	for alias, link := range cfg.Links {
		// Rule 1: alias must be unique (map guarantees this) and valid format
		if !aliasPattern.MatchString(alias) {
			errs.add(ErrInvalidAlias,
				fmt.Sprintf("link alias %q must match [a-zA-Z0-9-]", alias))
		}

		// Registration URL: as is MUST
		if link.As == "" {
			errs.add(ErrMissingAs,
				fmt.Sprintf("link %q: 'as' parameter is required", alias))
		}

		// Rule 12: keepalive must be positive integer, minimum 5 recommended
		if link.Keepalive < 1 {
			errs.add(ErrInvalidKeepalive,
				fmt.Sprintf("link %q: keepalive must be positive, got %d", alias, link.Keepalive))
		}

		// Rule 9: via (if present) must match host.alias:port
		if link.Via != nil {
			if !strings.Contains(link.Via.Host, ".") {
				errs.add(ErrInvalidVia,
					fmt.Sprintf("link %q: registration via must match host.alias:port, got %q",
						alias, link.Via.String()))
			}
		}
	}
}

// validateTunnels checks rules 2-8, 10-11, 13.
func validateTunnels(cfg *CanonicalConfig, errs *ValidationErrors) {
	idSet := make(map[string]bool)
	listenSet := make(map[string]bool)

	for i, t := range cfg.Tunnels {
		prefix := fmt.Sprintf("tunnel[%d] %q", i, t.Name)

		// Rule 2: id must be UUID and unique
		if t.ID == "" {
			errs.add(ErrInvalidTunnelID,
				fmt.Sprintf("%s: id is required", prefix))
		} else if !uuidPattern.MatchString(t.ID) {
			errs.add(ErrInvalidTunnelID,
				fmt.Sprintf("%s: id %q is not a valid UUID", prefix, t.ID))
		} else if idSet[t.ID] {
			errs.add(ErrDuplicateTunnelID,
				fmt.Sprintf("%s: tunnel id %q must be unique", prefix, t.ID))
		} else {
			idSet[t.ID] = true
		}

		// Rule 3: name must exist
		if t.Name == "" {
			errs.add(ErrMissingField,
				fmt.Sprintf("tunnel[%d]: name is required", i))
		}

		// Rule 4: listen and target must exist
		if t.Listen.RawURL == "" {
			errs.add(ErrMissingField,
				fmt.Sprintf("%s: listen is required", prefix))
		}
		if t.Target.RawURL == "" {
			errs.add(ErrMissingField,
				fmt.Sprintf("%s: target is required", prefix))
		}

		// Validate listen endpoint
		if t.Listen.RawURL != "" {
			validateTunnelEndpoint(cfg, prefix, "listen", t.Listen, errs)
		}

		// Validate target endpoint
		if t.Target.RawURL != "" {
			validateTunnelEndpoint(cfg, prefix, "target", t.Target, errs)
		}

		// Rule 10-11: each vtcp endpoint must carry authcode or authcodeRef
		if t.Listen.Kind == EndpointVNet && t.Listen.Authcode == "" && t.Listen.AuthcodeRef == "" {
			errs.add(ErrVtcpAuthRequired,
				fmt.Sprintf("%s.listen: authcode or authcodeRef is required for vtcp endpoint", prefix))
		}
		if t.Target.Kind == EndpointVNet && t.Target.Authcode == "" && t.Target.AuthcodeRef == "" {
			errs.add(ErrVtcpAuthRequired,
				fmt.Sprintf("%s.target: authcode or authcodeRef is required for vtcp endpoint", prefix))
		}

		// Rule 13: listen endpoint must not conflict
		if t.Listen.RawURL != "" {
			key := fmt.Sprintf("%s://%s", t.Listen.Scheme, t.Listen.Address())
			if listenSet[key] {
				errs.add(ErrListenConflict,
					fmt.Sprintf("%s: listen endpoint %s conflicts with another tunnel", prefix, key))
			} else {
				listenSet[key] = true
			}
		}
	}
}

// validateTunnelEndpoint checks rules 5-8 for a single tunnel endpoint.
func validateTunnelEndpoint(
	cfg *CanonicalConfig,
	prefix, role string,
	ep EndpointSpec,
	errs *ValidationErrors,
) {
	label := fmt.Sprintf("%s.%s", prefix, role)

	// Rule 7: tunnel endpoint must not carry via
	if hasViaParam(ep.RawURL) {
		errs.add(ErrTunnelViaNotAllowed,
			fmt.Sprintf("%s: via is only allowed in registration URL", label))
	}

	// Rule 8: tunnel endpoint must not use http/https/ws/wss
	if forbiddenTunnelSchemes[ep.Scheme] {
		errs.add(ErrInvalidScheme,
			fmt.Sprintf("%s: scheme %q is not allowed as tunnel endpoint", label, ep.Scheme))
	}

	// Rule 6: vtcp alias must exist in links
	if ep.Kind == EndpointVNet {
		if _, found := cfg.Links[ep.Alias]; !found {
			errs.add(ErrAliasNotFound,
				fmt.Sprintf("%s: alias %q not found in links", label, ep.Alias))
		}
	}
}

// hasViaParam checks if the raw URL contains a via query parameter.
func hasViaParam(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Query().Get("via") != ""
}
