package netx

// func Connect(config *agent.Config) (*flex.PacketConn, error) {
// 	if config.Agent.WsEnable {
// 		//
// 		// 使用Websocket协议连接
// 		//
// 		u := url.URL{
// 			Scheme: "ws",
// 			Host:   config.Agent.Address,
// 			Path:   config.Agent.WsPath,
// 		}
// 		if config.Agent.Wss {
// 			u.Scheme = "wss"
// 		}
// 		target := u.String()
// 		log.Printf("> connect '%v'\n", target)

// 		wsconn, _, err := websocket.DefaultDialer.Dial(target, nil)
// 		if err != nil {
// 			log.Printf("> dial websocket server failed: %v\n", err)
// 			return nil, err
// 		}
// 		return flex.NewWsPacketConn(wsconn), nil
// 	}

// 	//
// 	// 使用TCP连接
// 	//
// 	log.Printf("> connect '%v'\n", config.Agent.Address)
// 	conn, err := net.Dial("tcp4", config.Agent.Address)
// 	if err != nil {
// 		log.Printf("> dial tcp server failed: %v\n", err)
// 		return nil, err
// 	}

// 	// TCP连接需要进行加密操作
// 	if config.Agent.Password != "" {
// 		log.Printf("> make cipherconn\n")
// 		cc, err := cipherconn.New(conn, config.Agent.Password)
// 		if err != nil {
// 			log.Printf("> make cipherconn failed: %v\n", err)
// 			return nil, err
// 		}
// 		conn = cc
// 	}

// 	return flex.NewTcpPacketConn(conn), nil
// }
