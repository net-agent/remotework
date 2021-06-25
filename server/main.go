package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex"
	"github.com/net-agent/mixlisten"
)

func main() {
	var flags ServerFlags
	flags.Parse()

	// 读取配置
	log.Printf("read config from '%v'\n", flags.ConfigFileName)
	config, err := NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	// 初始化
	sw := flex.NewSwitcher(nil, config.Server.Password)

	log.Printf("try to listen on '%v'\n", config.Server.Listen)

	if !config.Server.WsEnable {
		sw.Run(config.Server.Listen)
		return
	}

	mxl := mixlisten.Listen("tcp", config.Server.Listen)
	mxl.Register(mixlisten.Flex())
	mxl.Register(mixlisten.HTTP())

	flexListener, err := mxl.GetListener(mixlisten.Flex().Name())
	if err != nil {
		log.Fatal("get flex listener failed: ", err)
	}

	httpListener, err := mxl.GetListener(mixlisten.HTTP().Name())
	if err != nil {
		log.Fatal("get http listener failed: ", err)
	}
	go serveFlex(sw, flexListener)
	go serveHTTP(sw, httpListener, config.Server.WsPath)

	mxl.Run()
	log.Println("server stopped")
}

func serveFlex(sw *flex.Switcher, listener net.Listener) {
	sw.Serve(listener)
	log.Println("flex server stopped.")
}

func serveHTTP(sw *flex.Switcher, listener net.Listener, wsPath string) {
	upgrader := websocket.Upgrader{}
	http.HandleFunc(wsPath, func(w http.ResponseWriter, r *http.Request) {
		wsconn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("upgrade failed: %v", err)))
			return
		}
		go sw.ServePacketConn(flex.NewWsPacketConn(wsconn))
	})
	log.Println("http server is running")
	http.Serve(listener, nil)
	log.Println("http server stopped.")
}
