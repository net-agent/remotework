// Package configv2 implements the v2.3 configuration model for the remotework agent.
//
// It provides parsing, normalization, and validation of link+tunnel based configuration,
// producing a CanonicalConfig that the runtime layer exclusively depends on.
//
// LoadAndCheck 采用逐项容错：每个 ValidationError 带 Scope/Item/ItemIndex，
// 坏 link/tunnel 会被精确剔除，好项继续运行；只有文件本身无法解析时才返回
// nil 配置（供 cmd 进入降级模式）。密钥引用（authRef/authcodeRef）不在此层
// 解析，运行时通过 utils.ResolveSecret 读取环境变量并在失败时 fail closed。
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
	ID      string `json:"id" toml:"id"`
	Name    string `json:"name" toml:"name"`
	Comment string `json:"comment" toml:"comment"` // 可选人类可读说明，不参与运行
	Listen  string `json:"listen" toml:"listen"`
	Target  string `json:"target" toml:"target"`
}

// PprofInfo controls the optional pprof HTTP server.
type PprofInfo struct {
	Enable bool   `json:"enable" toml:"enable"`
	Listen string `json:"listen" toml:"listen"`
}

// APIInfo controls the optional management API server.
type APIInfo struct {
	Enable              bool   `json:"enable" toml:"enable"`
	Listen              string `json:"listen" toml:"listen"`
	PollInterval        int    `json:"pollInterval" toml:"pollInterval"`
	StreamStatsInterval int    `json:"streamStatsInterval" toml:"streamStatsInterval"` // 数据流快照推送间隔（秒），缺省 1

	// AllowNonLoopback 显式允许 API 监听非回环地址。默认只允许回环地址，
	// 避免控制面在用户不知情时暴露到局域网/公网；非回环监听必须自行配置
	// API token，token 策略由 api 子系统负责。
	AllowNonLoopback bool `json:"allowNonLoopback" toml:"allowNonLoopback"`
}

// CanonicalConfig is the normalized, validated internal representation.
// The runtime layer MUST only depend on this structure.
type CanonicalConfig struct {
	Links   map[string]LinkSpec // alias -> LinkSpec
	Tunnels []TunnelSpec
	Pprof   PprofInfo
	API     APIInfo
}

// DefaultKeepalive 是 flex 运行时内部使用的默认心跳间隔（秒）。
// 配置层不接受显式 keepalive：flex v3.1.0 没有把该参数下发给 Session 的能力，
// 接受它只会造成“配置了但不生效”的静默 no-op。
const DefaultKeepalive = 15

// LinkSpec is the normalized representation of a link registration URL.
type LinkSpec struct {
	Alias     string   // local alias (key from links map)
	Scheme    string   // connection protocol (tcp, ws, wss, etc.)
	RelayURL  string   // full relay URL without query params and without userinfo
	As        string   // vhostname - node identity in virtual network
	Auth      string   // plain text auth (not recommended for production)
	AuthRef   string   // secret reference (env:NAME or bare env var name)
	Keepalive int      // 仅保留默认值 DefaultKeepalive；显式配置已拒绝
	Via       *ViaSpec // Deprecated: flex v3.1.0 不支持上游路由，Validate 会拒绝
}

// ViaSpec describes an upstream relay endpoint for link establishment.
//
// Deprecated: flex v3.1.0 没有实现 via 上游路由，Normalize 会直接拒绝该参数；
// 保留类型仅用于对历史/程序化构造的 LinkSpec 做防御性校验。
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

	// authcode 从 URL query 提取（?authcode= / ?authcodeRef=），所有 scheme 都会
	// 保留原始值以便校验与诊断；只有 vtcp 端点允许使用：
	//   - 物理 tcp/tcp4/tcp6 端点携带 authcode 会被 Validate 拒绝（否则用户会
	//     误以为已加密，实际仍是明文）；
	//   - 同时给出 authcode 与 authcodeRef 时，运行时以 authcode 为准；
	//   - authcodeRef 由 utils.ResolveSecret 解析（env:NAME 或裸环境变量名），
	//     解析失败必须失败关闭，绝不能把引用字面量当密钥。
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
