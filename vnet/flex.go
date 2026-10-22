package vnet

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v3/node"
	"github.com/net-agent/flex/v3/packet"
	"github.com/net-agent/flex/v3/packet/ws"
	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/utils"
)

var (
	ErrNodeClosed = errors.New("connect failed, node closed")
)

// flexDialTimeout 是 link 建连（TCP 拨号 / WebSocket 握手）的超时
const flexDialTimeout = 10 * time.Second

// flexWSDialer 独立的 WebSocket 拨号器：不修改全局 websocket.DefaultDialer
var flexWSDialer = &websocket.Dialer{
	Proxy:            http.ProxyFromEnvironment,
	HandshakeTimeout: flexDialTimeout,
}

// FlexLinkConfig holds the parameters needed to create a flex network from a link spec.
type FlexLinkConfig struct {
	Alias    string // local alias, used as the network name
	Scheme   string // connection protocol (tcp, ws, wss, etc.)
	RelayURL string // full relay URL (without query params)
	Domain   string // node identity in virtual network (the "as" value)
	Auth     string // authentication secret
	// Logger 可选，Session 日志器；为 nil 时回退到按别名自建的模块日志器
	Logger *slog.Logger
	// OnStateChange 可选回调，Session 状态变更时触发（参数为状态字符串）
	OnStateChange func(oldState, newState string)
	// OnStreamOpen / OnStreamClosed 可选回调，流打开 / 完全关闭时触发（flex Trace 钩子，
	// 每次重连新建的 Node 会自动继承）。注意：Closed 在流完全关闭后的滞留期
	// （flex DefaultDetachRetention，约 2 秒）届满时触发，非即时。
	OnStreamOpen   func(st *stream.State)
	OnStreamClosed func(st *stream.State)
}

type flexNetwork struct {
	name    string // network name (alias in v2)
	meta    flexMeta
	session *node.Session
	tracker streamStateTracker
}

// flexMeta stores metadata for reporting.
type flexMeta struct {
	scheme  string
	address string
	domain  string
}

// NewFlexNetworkFromLink creates a flex network from v2 link configuration.
func NewFlexNetworkFromLink(cfg FlexLinkConfig) (Network, error) {
	if cfg.Alias == "" {
		return nil, errors.New("flex: alias is required")
	}
	if cfg.Domain == "" {
		return nil, errors.New("flex: domain (as) is required")
	}

	connector, address, err := buildConnector(cfg.Scheme, cfg.RelayURL)
	if err != nil {
		return nil, fmt.Errorf("flex: %w", err)
	}

	sessCfg := node.SessionConfig{
		Domain:   cfg.Domain,
		Password: cfg.Auth,
		Mac:      utils.GetMacAddressStr(), // 设备标识，握手时上报给 switcher
	}

	sess := node.NewSession(connector, sessCfg)
	log := cfg.Logger
	if log == nil {
		log = utils.NewModuleLogger("vnet.flex." + cfg.Alias)
	}
	sess.SetLogger(log)

	fair := node.DefaultSchedulerConfig()
	log.Info("enabling fair connection scheduling",
		"enabled", true,
		"policy", nodeSchedulerPolicyName(fair.Policy),
		"quantumBytes", fair.QuantumBytes,
		"minChargeBytes", fair.MinChargeBytes,
		"quantumPackets", fair.QuantumPackets,
	)

	if cfg.OnStreamOpen != nil || cfg.OnStreamClosed != nil {
		// SetTrace 需在 Serve 前调用；重连后新建的 Node 由 Session 自动注入同一组钩子
		sess.SetTrace(&node.Trace{
			StreamOpen:   cfg.OnStreamOpen,
			StreamClosed: cfg.OnStreamClosed,
		})
	}

	if cfg.OnStateChange != nil {
		// SessionState 枚举定义见 github.com/net-agent/flex/v3/node/session.go
		// 状态流转: Ready → Idle → Connecting → Online ↔ Connecting → Closed
		//   Ready      — Session 已创建，Serve 尚未启动
		//   Idle       — 已进入 Serve loop，等待首次 Listen/Dial 触发
		//   Connecting — 连接中或断线重连中
		//   Online     — 已成功连接，Node 可用
		//   Closed     — Session 已永久关闭
		sess.OnStateChange(func(old, new_ node.SessionState) {
			cfg.OnStateChange(old.String(), new_.String())
		})
	}

	go sess.Serve()

	return &flexNetwork{
		name: cfg.Alias,
		meta: flexMeta{
			scheme:  cfg.Scheme,
			address: address,
			domain:  cfg.Domain,
		},
		session: sess,
	}, nil
}

// buildConnector creates a packet.Conn factory based on the connection scheme.
func buildConnector(scheme, relayURL string) (func() (packet.Conn, error), string, error) {
	switch scheme {
	case "ws", "wss":
		return func() (packet.Conn, error) {
			wsConn, _, err := flexWSDialer.Dial(relayURL, nil)
			if err != nil {
				return nil, err
			}
			return ws.NewConn(wsConn), nil
		}, relayURL, nil

	case "tcp", "":
		// Parse relay URL to get host:port for TCP dial
		u, err := url.Parse(relayURL)
		if err != nil {
			return nil, "", fmt.Errorf("invalid relay URL %q: %w", relayURL, err)
		}
		addr := u.Host
		return func() (packet.Conn, error) {
			conn, err := net.DialTimeout("tcp", addr, flexDialTimeout)
			if err != nil {
				return nil, err
			}
			return packet.NewWithConn(conn), nil
		}, addr, nil

	default:
		return nil, "", fmt.Errorf("unsupported scheme %q", scheme)
	}
}

func (fnet *flexNetwork) GetName() string                             { return fnet.name }
func (fnet *flexNetwork) Dial(network, addr string) (net.Conn, error) { return fnet.session.Dial(addr) }
func (fnet *flexNetwork) Stop()                                       { fnet.session.Close() }

func (fnet *flexNetwork) Listen(network, addr string) (net.Listener, error) {
	port, err := parsePort(addr)
	if err != nil {
		return nil, err
	}
	return fnet.session.Listen(port)
}

func parsePort(addr string) (uint16, error) {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}
	if port < 0 || port > 65535 {
		return 0, errors.New("invalid port number")
	}
	return uint16(port), nil
}

func (fnet *flexNetwork) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	pinger := fnet.session.GetNode()
	if pinger == nil {
		return 0, ErrNodeClosed
	}
	return pinger.PingDomain(domain, timeout)
}

// GetStreamStates 实现 registry 的 streamStateProvider 可选接口。
// 重连中（GetNode 为 nil）时活跃流为空，仅返回已关闭流历史。
func (fnet *flexNetwork) GetStreamStates() (actives, closeds []*stream.State) {
	n := fnet.session.GetNode()
	if n == nil {
		// 注意：nil *node.Node 不能直接装箱进 interface，需显式传 nil
		return fnet.tracker.collect(nil)
	}
	return fnet.tracker.collect(n)
}

// CloseStream 实现 StreamCloser 可选接口：按流 State.Index 手动关闭一条活跃流。
// 重连中（GetNode 为 nil）返回 false。
func (fnet *flexNetwork) CloseStream(index int32) bool {
	n := fnet.session.GetNode()
	if n == nil {
		return false
	}
	return n.CloseStream(index)
}

func (fnet *flexNetwork) Meta() NetworkMeta {
	meta := NetworkMeta{
		Protocol:   fnet.meta.scheme,
		Address:    fnet.meta.address,
		Domain:     fnet.meta.domain,
		State:      fnet.session.GetState().String(),
		LastErr:    fnet.session.GetLastErr(),
		Reconnects: fnet.session.GetReconnectCount(),
		Kind:       "virtual",
	}
	// 重连中（GetNode 为 nil）节点级指标保持零值
	if n := fnet.session.GetNode(); n != nil {
		meta.BytesRead, meta.BytesWritten = n.GetReadWriteSize()
		meta.RTT = n.GetLastRTT()
	}
	return meta
}

// nodeSchedulerPolicyName 把 flex node 的调度策略枚举转成可读名称。
func nodeSchedulerPolicyName(p node.SchedulerPolicy) string {
	if p == node.SchedulerPolicyPackets {
		return "packets"
	}
	return "cost-budget"
}
