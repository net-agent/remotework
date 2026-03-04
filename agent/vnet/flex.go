package vnet

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v3/node"
	"github.com/net-agent/flex/v3/packet"
	"github.com/net-agent/flex/v3/packet/ws"
)

var (
	ErrNodeClosed = errors.New("connect failed, node closed")
)

// FlexLinkConfig holds the parameters needed to create a flex network from a link spec.
type FlexLinkConfig struct {
	Alias    string // local alias, used as the network name
	Scheme   string // connection protocol (tcp, ws, wss, etc.)
	RelayURL string // full relay URL (without query params)
	Domain   string // node identity in virtual network (the "as" value)
	Auth     string // authentication secret
	// Keepalive is available for future use
}

type flexNetwork struct {
	name    string // network name (alias in v2)
	meta    flexMeta
	session *node.Session
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
	}

	sess := node.NewSession(connector, sessCfg)
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
			wsConn, _, err := websocket.DefaultDialer.Dial(relayURL, nil)
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
			conn, err := net.Dial("tcp", addr)
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

func (fnet *flexNetwork) Meta() NetworkMeta {
	state := "connecting"
	if fnet.session.GetNode() != nil {
		state = "online"
	}
	return NetworkMeta{
		Protocol: fnet.meta.scheme,
		Address:  fnet.meta.address,
		Domain:   fnet.meta.domain,
		State:    state,
	}
}
