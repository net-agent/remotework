package main

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v2/packet"
	"github.com/net-agent/flex/v2/switcher"
)

func ServeWs(app *switcher.Server, info ServerInfo, listener net.Listener) {
	r := mux.NewRouter()
	r.Methods("GET").Path(info.WsPath).HandlerFunc(GetWsHandler(app))
	http.Serve(listener, r)
}

func GetWsHandler(app *switcher.Server) http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			ret := fmt.Sprintf("upgrade failed: %v", err)
			w.Write([]byte(ret))
			return
		}

		pc := packet.NewWithWs(c)
		syslog.Printf("ws  agent connected, remote=%v\n", c.RemoteAddr())
		go app.HandlePacketConn(pc)
	}
}
