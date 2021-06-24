package main

import (
	"log"
	"net"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/net-agent/cipherconn"
	"github.com/net-agent/flex"
)

var aliveHost *flex.Host
var getHostLocker sync.Mutex

func getHost() (*flex.Host, error) {
	getHostLocker.Lock()
	defer getHostLocker.Unlock()

	if aliveHost != nil {
		return aliveHost, nil
	}

	// 创建连接（PacketConn)
	var pconn *flex.PacketConn
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
			log.Printf("dial websocket server failed: %v\n", err)
			return nil, err
		}
		pconn = flex.NewWsPacketConn(wsconn)
	} else {
		//
		// 使用TCP连接
		//
		log.Printf("> connect '%v'\n", config.Agent.Address)
		conn, err := net.Dial("tcp4", config.Agent.Address)
		if err != nil {
			log.Printf("dial tcp server failed: %v\n", err)
			return nil, err
		}

		// TCP连接需要进行加密操作
		if config.Agent.Password != "" {
			log.Printf("> make cipherconn\n")
			cc, err := cipherconn.New(conn, config.Agent.Password)
			if err != nil {
				log.Printf("make cipherconn failed: %v\n", err)
				return nil, err
			}
			conn = cc
		}

		pconn = flex.NewTcpPacketConn(conn)
	}

	// 协议转换
	log.Printf("> upgrade to host, domain='%v'\n", config.Agent.Domain)
	host, err := flex.UpgradeToHost(pconn, &flex.HostRequest{
		Domain: config.Agent.Domain,
		Mac:    "test-mac-token",
	})
	if err != nil {
		log.Printf("upgrade failed: %v\n", err)
		return nil, err
	}

	log.Printf("> host created, ip=%v\n", host.IP())

	aliveHost = host
	return host, nil
}
