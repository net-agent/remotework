package server

import (
	"net"
	"time"

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
		syslog.Printf("tcp agent connected, remote=%v\n", c.RemoteAddr())
		// Define callbacks for logging
		onStart := func(ctx *switcher.Context) {
			syslog.Printf("tcp agent connected: domain='%v' ip='%v'\n", ctx.Domain, ctx.IP)
		}
		onStop := func(ctx *switcher.Context, duration time.Duration) {
			syslog.Printf("tcp agent disconnected: domain='%v' ip='%v' duration='%v'\n", ctx.Domain, ctx.IP, duration)
		}

		go app.HandlePacketConn(pc, onStart, onStop)
	}
}
