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
		syslog.Info("tcp agent connected", "remote", c.RemoteAddr())
		// Define callbacks for logging
		onStart := func(ctx *switcher.Context) {
			syslog.Info("tcp agent connected", "domain", ctx.Domain, "ip", ctx.IP)
		}
		onStop := func(ctx *switcher.Context, duration time.Duration) {
			syslog.Info("tcp agent disconnected", "domain", ctx.Domain, "ip", ctx.IP, "duration", duration)
		}

		go app.HandlePacketConn(pc, onStart, onStop)
	}
}
