package main

import (
	"log"
	"net"

	"github.com/net-agent/flex/v2/packet"
	"github.com/net-agent/flex/v2/switcher"
)

func ServeTCP(app *switcher.Server, info ServerInfo, listener net.Listener) {
	for {
		c, err := listener.Accept()
		if err != nil {
			return
		}

		pc := packet.NewWithConn(c)
		log.Printf("tcp agent connected, remote=%v\n", c.RemoteAddr())
		go app.ServeConn(pc)
	}
}
