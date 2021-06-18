package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/net-agent/flex"
)

func dial(host *flex.Host, addr string) (net.Conn, error) {
	network, hostname, portStr, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseInt(portStr, 10, 16)
	if err != nil {
		return nil, err
	}

	rawAddr := fmt.Sprintf("%v:%v", hostname, port)
	if network == "tcp" || network == "tcp4" || network == "tcp6" {
		return net.Dial(network, rawAddr)
	}

	if host != nil {
		return nil, errors.New("flex.Host is nil")
	}
	return host.Dial(0, uint16(port))
}

func listen(host *flex.Host, addr string) {

}

func parseAddr(addr string) (network, host string, port uint16, err error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", 0, err
	}
	intPort, err = strconv.ParseInt(u.Port(), 10, 16)
	network, host = u.Scheme, u.Host

	if network == "" {
		network = "tcp4"
	}

	if host == "" {
		host = "localhost"
	}

	if port == "" {
		port = "80"
	}

	return
}
