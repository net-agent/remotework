package main

import (
	"log"
	"net"

	"github.com/net-agent/flex"
)

func main() {
	conn, err := net.Dial("tcp4", "localhost:2038")
	if err != nil {
		log.Fatal("dial failed", err)
	}

	_, err = flex.UpgradeToHost(conn, &flex.HostRequest{
		Domain: "test",
		Mac:    "test-mac-token",
	})
	if err != nil {
		log.Fatal("upgrade failed", err)
	}

	net.Listen("tcp4", "localhost:1000")
}
