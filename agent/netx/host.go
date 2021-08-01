package netx

import (
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/cipherconn"
	"github.com/net-agent/remotework/agent"
)

const (
	EvConnected    = "connected"
	EvDisconnected = "disconnected"
)

var globalHost *flex.Host
var globalInitOnce sync.Once

func GetHost() *flex.Host {
	return globalHost
}

func Connect(config *agent.Config) (*flex.PacketConn, error) {
	if config.Agent.WsEnable {
		//
		// 使用Websocket协议连接
		//
		u := url.URL{
			Scheme: "ws",
			Host:   config.Agent.Address,
			Path:   config.Agent.WsPath,
		}
		if config.Agent.Wss {
			u.Scheme = "wss"
		}
		target := u.String()
		log.Printf("> connect '%v'\n", target)

		wsconn, _, err := websocket.DefaultDialer.Dial(target, nil)
		if err != nil {
			log.Printf("> dial websocket server failed: %v\n", err)
			return nil, err
		}
		return flex.NewWsPacketConn(wsconn), nil
	}

	//
	// 使用TCP连接
	//
	log.Printf("> connect '%v'\n", config.Agent.Address)
	conn, err := net.Dial("tcp4", config.Agent.Address)
	if err != nil {
		log.Printf("> dial tcp server failed: %v\n", err)
		return nil, err
	}

	// TCP连接需要进行加密操作
	if config.Agent.Password != "" {
		log.Printf("> make cipherconn\n")
		cc, err := cipherconn.New(conn, config.Agent.Password)
		if err != nil {
			log.Printf("> make cipherconn failed: %v\n", err)
			return nil, err
		}
		conn = cc
	}

	return flex.NewTcpPacketConn(conn), nil
}

func KeepHostAlive(config *agent.Config, hostReady chan string) {
	var err error
	var pc *flex.PacketConn
	var host *flex.Host
	var ctxid uint64
	var failCount int = 0

	waitDur := time.Duration(0)
	for {
		if waitDur > 0 {
			log.Printf("> reconnect after %v\n", waitDur)
			<-time.After(waitDur)
		}

		pc, err = Connect(config)
		if err != nil {
			waitDur = time.Second * 15
			continue
		}

		// 协议转换
		log.Printf("> upgrade to host, domain='%v'\n", config.Agent.Domain)
		var newhost *flex.Host
		var newCtxid uint64
		newhost, newCtxid, err = flex.UpgradeToHost(pc, &flex.HostRequest{
			Domain: config.Agent.Domain,
			Mac:    "test-mac-token",
			Ctxid:  ctxid,
		}, false)

		switch err {

		case nil:
			log.Printf("> connected\n")
			host = newhost
			ctxid = newCtxid
			globalHost = host
			failCount = 0
			// 回调，通知调用者

			hostReady <- EvConnected

		case flex.ErrReconnected:
			log.Printf("> reconnected, ctxid=%v\n", ctxid)
			host.Replace(pc)
			failCount = 0
			// 回调，通知调用者

			hostReady <- EvConnected

		default:
			log.Printf("> upgrade failed: %v\n", err)
			failCount++
			if failCount > 5 {
				ctxid = 0
			}
			waitDur = time.Second * 15
			continue
		}

		// 运行
		host.Run()

		hostReady <- EvDisconnected
		<-time.After(time.Second) // 等待svc把log输出

		log.Println("> disconnected, try to reconnect...")
		waitDur = time.Second * 3
	}
}
