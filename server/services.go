package main

import (
	"net"

	"github.com/net-agent/flex"
)

func RegistHost(sw *flex.Switcher, domain string) (*flex.Host, error) {
	c1, c2 := net.Pipe()

	pc1 := flex.NewTcpPacketConn(c1)
	pc2 := flex.NewTcpPacketConn(c2)
	go sw.ServePacketConn(pc2)

	host, _, err := flex.UpgradeToHost(pc1, &flex.HostRequest{
		Domain: domain,
		Ctxid:  0,
		Mac:    "xx",
	}, true)

	return host, err
}
