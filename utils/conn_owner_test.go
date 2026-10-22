package utils

import (
	"testing"

	gopsnet "github.com/shirou/gopsutil/v4/net"
)

func TestBuildTCPLocalEndpoints(t *testing.T) {
	connections := []gopsnet.ConnectionStat{
		{
			// 客户端连接的临时端口，属主为客户端进程
			Status: "ESTABLISHED",
			Type:   1,
			Pid:    100,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 64443},
			Raddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 1080},
		},
		{
			// 同端点的 TIME_WAIT 残留（pid 未知），不应覆盖 ESTABLISHED 行
			Status: "TIME_WAIT",
			Type:   1,
			Pid:    0,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 64443},
		},
		{
			// 监听行同样登记（键为监听端口，不与临时端口冲突）
			Status: "LISTEN",
			Type:   1,
			Pid:    42,
			Laddr:  gopsnet.Addr{IP: "0.0.0.0", Port: 1080},
		},
		{
			// 端口非法，跳过
			Status: "LISTEN",
			Type:   1,
			Pid:    1,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 65536},
		},
		{
			// IPv6 loopback
			Status: "ESTABLISHED",
			Type:   1,
			Pid:    101,
			Laddr:  gopsnet.Addr{IP: "::1", Port: 5000},
		},
		{
			// IPv4-in-IPv6 映射地址归一化为 IPv4
			Status: "ESTABLISHED",
			Type:   1,
			Pid:    102,
			Laddr:  gopsnet.Addr{IP: "::ffff:127.0.0.1", Port: 6000},
		},
	}

	got := buildTCPLocalEndpoints(connections)
	want := map[TCPEndpointKey]int32{
		{IP: "127.0.0.1", Port: 64443}: 100,
		{IP: "0.0.0.0", Port: 1080}:    42,
		{IP: "::1", Port: 5000}:        101,
		{IP: "127.0.0.1", Port: 6000}:  102,
	}
	if len(got) != len(want) {
		t.Fatalf("len(buildTCPLocalEndpoints()) = %d, want %d (%v)", len(got), len(want), got)
	}
	for key, pid := range want {
		if got[key] != pid {
			t.Fatalf("got[%+v] = %d, want %d", key, got[key], pid)
		}
	}
}

func TestBuildTCPLocalEndpointsConflictOrder(t *testing.T) {
	// TIME_WAIT（pid 未知）先出现、ESTABLISHED 后出现：后者覆盖
	connections := []gopsnet.ConnectionStat{
		{Status: "TIME_WAIT", Type: 1, Pid: 0, Laddr: gopsnet.Addr{IP: "127.0.0.1", Port: 7000}},
		{Status: "ESTABLISHED", Type: 1, Pid: 200, Laddr: gopsnet.Addr{IP: "127.0.0.1", Port: 7000}},
	}
	got := buildTCPLocalEndpoints(connections)
	if pid := got[TCPEndpointKey{IP: "127.0.0.1", Port: 7000}]; pid != 200 {
		t.Fatalf("ESTABLISHED should win over TIME_WAIT, got pid %d", pid)
	}

	// 两条都非 ESTABLISHED：PID 合法者优先
	connections = []gopsnet.ConnectionStat{
		{Status: "TIME_WAIT", Type: 1, Pid: 0, Laddr: gopsnet.Addr{IP: "127.0.0.1", Port: 7001}},
		{Status: "CLOSE_WAIT", Type: 1, Pid: 201, Laddr: gopsnet.Addr{IP: "127.0.0.1", Port: 7001}},
	}
	got = buildTCPLocalEndpoints(connections)
	if pid := got[TCPEndpointKey{IP: "127.0.0.1", Port: 7001}]; pid != 201 {
		t.Fatalf("valid pid should win among non-ESTABLISHED rows, got pid %d", pid)
	}
}

func TestNormalizeIPText(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1":        "127.0.0.1",
		"::1":              "::1",
		"::ffff:127.0.0.1": "127.0.0.1",
		"0.0.0.0":          "0.0.0.0",
		"not-an-ip":        "not-an-ip", // 无法解析原样返回
	}
	for in, want := range tests {
		if got := normalizeIPText(in); got != want {
			t.Fatalf("normalizeIPText(%q) = %q, want %q", in, got, want)
		}
	}
}
