package agent

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v3/node"
	"github.com/net-agent/flex/v3/packet"
)

var (
	ErrNodeClosed = errors.New("connect failed, node closed")
)

type flexNetwork struct {
	hub     *Hub
	info    AgentInfo
	session *node.Session
}

func NewNetwork(hub *Hub, info AgentInfo) (Network, error) {
	var connector func() (packet.Conn, error)

	switch info.Protocol {
	case "ws", "wss":
		wsURL := fmt.Sprintf("%s://%s%s", info.Protocol, info.Address, info.WsPath)
		connector = func() (packet.Conn, error) {
			wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				return nil, err
			}
			return packet.NewWithWs(wsConn), nil
		}
	default:
		connector = func() (packet.Conn, error) {
			conn, err := net.Dial("tcp", info.Address)
			if err != nil {
				return nil, err
			}
			return packet.NewWithConn(conn), nil
		}
	}

	cfg := node.SessionConfig{
		Domain:   info.Domain,
		Password: info.Password,
	}

	sess := node.NewSession(connector, cfg)
	go sess.Serve()

	return &flexNetwork{
		hub:     hub,
		info:    info,
		session: sess,
	}, nil
}

func (fnet *flexNetwork) GetName() string                             { return fnet.info.Name }
func (fnet *flexNetwork) Dial(network, addr string) (net.Conn, error) { return fnet.session.Dial(addr) }
func (fnet *flexNetwork) Stop()                                       { fnet.session.Close() }

func (fnet *flexNetwork) Listen(network, addr string) (net.Listener, error) {
	if network != fnet.GetName() {
		return nil, errors.New("network name mismatch")
	}
	port, err := parsePort(addr)
	if err != nil {
		return nil, err
	}
	return fnet.session.Listen(port)
}

func parsePort(addr string) (uint16, error) {
	hostname, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	if hostname != "" && hostname != "0" && hostname != "local" && hostname != "localhost" {
		return 0, errors.New("invalid listen hostname")
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
	return NetworkMeta{
		Protocol: fnet.info.Protocol,
		Address:  fnet.info.Address,
		Domain:   fnet.info.Domain,
	}
}
