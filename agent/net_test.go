package main

import "testing"

func TestParseAddr(t *testing.T) {
	testcases := []struct {
		addr    string
		network string
		host    string
		port    uint16
	}{
		{"192.168.1.1:1080", "tcp", "192.168.1.1", 1080},
		{"tcp://192.168.1.1:1080", "tcp", "192.168.1.1", 1080},
		{"tcp://192.168.1.1:0", "tcp", "192.168.1.1", 0},
	}

	for _, tc := range testcases {
		network, host, port, err := parseAddr(tc.addr)
		if err != nil {
			t.Error(err)
			return
		}
		if network != tc.network {
			t.Error("network not equal", network, tc.network)
			return
		}
		if host != tc.host {
			t.Error("host not equal", host, tc.host)
			return
		}
		if port != tc.port {
			t.Error("port not equal", port, tc.port)
			return
		}
	}
}
