// Package configv2 implements the v2.3 configuration model for the remotework agent.
//
// It provides parsing, normalization, and validation of link+tunnel based configuration,
// producing a CanonicalConfig that the runtime layer exclusively depends on.
package configv2

import "fmt"

// RawConfig is the user-facing configuration structure loaded from JSON/TOML files.
type RawConfig struct {
	Links   map[string]string `json:"links" toml:"links"`     // alias -> Registration URL
	Tunnels []TunnelRaw       `json:"tunnels" toml:"tunnels"` // tunnel rules
	Pprof   PprofInfo         `json:"pprof" toml:"pprof"`
	API     APIInfo           `json:"api" toml:"api"`
}

// TunnelRaw is the raw tunnel configuration as written by the user.
type TunnelRaw struct {
	ID     string `json:"id" toml:"id"`
	Name   string `json:"name" toml:"name"`
	Listen string `json:"listen" toml:"listen"`
	Target string `json:"target" toml:"target"`
}

// PprofInfo controls the optional pprof HTTP server.
type PprofInfo struct {
	Enable bool   `json:"enable" toml:"enable"`
	Listen string `json:"listen" toml:"listen"`
}

// APIInfo controls the optional management API server.
type APIInfo struct {
	Enable       bool   `json:"enable" toml:"enable"`
	Listen       string `json:"listen" toml:"listen"`
	PollInterval int    `json:"pollInterval" toml:"pollInterval"`
}

// CanonicalConfig is the normalized, validated internal representation.
// The runtime layer MUST only depend on this structure.
type CanonicalConfig struct {
	Links   map[string]LinkSpec // alias -> LinkSpec
	Tunnels []TunnelSpec
	Pprof   PprofInfo
	API     APIInfo
}

// LinkSpec is the normalized representation of a link registration URL.
type LinkSpec struct {
	Alias     string  // local alias (key from links map)
	Scheme    string  // connection protocol (tcp, ws, wss, etc.)
	RelayURL  string  // full relay URL without query params
	As        string  // vhostname - node identity in virtual network
	Auth      string  // plain text auth (not recommended for production)
	AuthRef   string  // auth reference (env var name, secret manager key)
	Keepalive int     // heartbeat interval in seconds, default 15
	Via       *ViaSpec // optional upstream relay routing
}

// ViaSpec describes an upstream relay endpoint for link establishment.
type ViaSpec struct {
	Host string // vhostname.alias
	Port string // port number
}

// String returns the via spec as "host:port".
func (v *ViaSpec) String() string {
	return fmt.Sprintf("%s:%s", v.Host, v.Port)
}

// TunnelSpec is the normalized representation of a tunnel rule.
type TunnelSpec struct {
	ID     string
	Name   string
	Listen EndpointSpec
	Target EndpointSpec
}

// EndpointKind categorizes the endpoint type.
type EndpointKind int

const (
	EndpointNet     EndpointKind = iota // tcp:// - physical network
	EndpointVNet                        // vtcp:// - virtual network
	EndpointBuiltin                     // socks5:// etc. - built-in service
)

// String returns the human-readable endpoint kind name.
func (k EndpointKind) String() string {
	switch k {
	case EndpointNet:
		return "net"
	case EndpointVNet:
		return "vnet"
	case EndpointBuiltin:
		return "builtin"
	default:
		return "unknown"
	}
}

// EndpointSpec is the normalized representation of a tunnel endpoint.
type EndpointSpec struct {
	Kind   EndpointKind
	Scheme string // tcp, vtcp, socks5, etc.
	Host   string // For tcp: IP/hostname; For vtcp: vhostname.alias
	Port   string // port number

	// vtcp-specific fields (populated only when Kind == EndpointVNet)
	VHostname string // node identity extracted from vtcp address
	Alias     string // link alias extracted from vtcp address

	// authcode extracted from URL query (?authcode= / ?authcodeRef=)
	Authcode    string
	AuthcodeRef string

	// builtin-specific fields (populated only when Kind == EndpointBuiltin)
	Username string
	Password string

	RawURL string // original URL string for debugging
}

// Address returns "host:port" representation.
func (e EndpointSpec) Address() string {
	if e.Port == "" {
		return e.Host
	}
	return fmt.Sprintf("%s:%s", e.Host, e.Port)
}

// IsVNet returns true if this endpoint is a virtual network endpoint.
func (e EndpointSpec) IsVNet() bool {
	return e.Kind == EndpointVNet
}
