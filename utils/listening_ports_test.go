package utils

import (
	"testing"

	gopsnet "github.com/shirou/gopsutil/v4/net"
)

func TestIsListeningConnection(t *testing.T) {
	tests := []struct {
		name string
		conn gopsnet.ConnectionStat
		want bool
	}{
		{
			name: "LISTEN status",
			conn: gopsnet.ConnectionStat{Status: "LISTEN"},
			want: true,
		},
		{
			name: "LISTENING status",
			conn: gopsnet.ConnectionStat{Status: "LISTENING"},
			want: true,
		},
		{
			name: "case insensitive",
			conn: gopsnet.ConnectionStat{Status: "listen"},
			want: true,
		},
		{
			name: "established ignored",
			conn: gopsnet.ConnectionStat{Status: "ESTABLISHED"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isListeningConnection(tt.conn); got != tt.want {
				t.Fatalf("isListeningConnection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildListeningPorts(t *testing.T) {
	// 替换进程启动时间查询，避免单测触发真实系统调用；同时统计调用次数验证 pid 缓存
	calls := make(map[int32]int)
	origin := getProcessStartMs
	getProcessStartMs = func(pid int32) int64 {
		calls[pid]++
		return 1700000000000 + int64(pid)
	}
	t.Cleanup(func() { getProcessStartMs = origin })

	connections := []gopsnet.ConnectionStat{
		{
			Status: "LISTEN",
			Type:   1,
			Pid:    100,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 8080},
		},
		{
			Status: "ESTABLISHED",
			Type:   1,
			Pid:    101,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 8081},
		},
		{
			// 同端口同地址但 pid 不同（SO_REUSEPORT），不应合并
			Status: "LISTEN",
			Type:   1,
			Pid:    102,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 8080},
		},
		{
			Status: "LISTEN",
			Type:   1,
			Pid:    103,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 65536},
		},
		{
			Status: "LISTENING",
			Type:   1,
			Pid:    104,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 22},
		},
		{
			// 同端口同 pid 但地址不同（IPv4/IPv6 双栈），不应合并
			Status: "LISTEN",
			Type:   1,
			Pid:    100,
			Laddr:  gopsnet.Addr{IP: "0.0.0.0", Port: 8080},
		},
		{
			// 与第一条完全相同，应被去重
			Status: "LISTEN",
			Type:   1,
			Pid:    100,
			Laddr:  gopsnet.Addr{IP: "127.0.0.1", Port: 8080},
		},
	}

	got := buildListeningPorts(connections)
	if len(got) != 4 {
		t.Fatalf("len(buildListeningPorts()) = %d, want 4", len(got))
	}
	// 排序：端口升序，同端口按地址字典序，再按 PID 升序
	wantKeys := []struct {
		port    uint16
		address string
		pid     uint32
	}{
		{22, "127.0.0.1", 104},
		{8080, "0.0.0.0", 100},
		{8080, "127.0.0.1", 100},
		{8080, "127.0.0.1", 102},
	}
	for i, want := range wantKeys {
		if got[i].Port != want.port || got[i].Address != want.address || got[i].PID != want.pid {
			t.Fatalf("got[%d] = {Port: %d, Address: %q, PID: %d}, want {Port: %d, Address: %q, PID: %d}",
				i, got[i].Port, got[i].Address, got[i].PID, want.port, want.address, want.pid)
		}
	}
	// Address 与 ProcStartMs 应被填充
	if got[1].ProcStartMs != 1700000000100 {
		t.Fatalf("ProcStartMs = %d, want %d", got[1].ProcStartMs, 1700000000100)
	}
	// pid 100 出现 3 次，但启动时间只应查询一次（缓存生效）
	if calls[100] != 1 {
		t.Fatalf("getProcessStartMs calls for pid 100 = %d, want 1", calls[100])
	}
}

func TestGetProcessStartMsInvalidPID(t *testing.T) {
	// pid 非法时直接返回 0，不触发系统调用
	if got := getProcessStartMs(0); got != 0 {
		t.Fatalf("getProcessStartMs(0) = %d, want 0", got)
	}
	if got := getProcessStartMs(-1); got != 0 {
		t.Fatalf("getProcessStartMs(-1) = %d, want 0", got)
	}
}

func TestDedupeListeningPorts(t *testing.T) {
	items := []ListeningPortInfo{
		{Port: 8080, Protocol: "tcp", PID: 1, ProcessName: "a", Address: "127.0.0.1"},
		{Port: 22, Protocol: "tcp", PID: 2, ProcessName: "ssh", Address: "0.0.0.0"},
		// 同端口同地址但 pid 不同，不合并
		{Port: 8080, Protocol: "tcp", PID: 3, ProcessName: "b", Address: "127.0.0.1"},
		// 同端口同 pid 但地址不同，不合并
		{Port: 8080, Protocol: "tcp", PID: 1, ProcessName: "a", Address: "::"},
		// 与第一条完全相同，合并
		{Port: 8080, Protocol: "tcp", PID: 1, ProcessName: "a", Address: "127.0.0.1"},
	}

	got := dedupeListeningPorts(items)
	if len(got) != 4 {
		t.Fatalf("len(dedupeListeningPorts()) = %d, want 4", len(got))
	}
	// 排序确定性：端口升序，同端口地址字典序（"127.0.0.1" < "::"），再按 PID 升序
	want := []ListeningPortInfo{
		{Port: 22, Protocol: "tcp", PID: 2, ProcessName: "ssh", Address: "0.0.0.0"},
		{Port: 8080, Protocol: "tcp", PID: 1, ProcessName: "a", Address: "127.0.0.1"},
		{Port: 8080, Protocol: "tcp", PID: 3, ProcessName: "b", Address: "127.0.0.1"},
		{Port: 8080, Protocol: "tcp", PID: 1, ProcessName: "a", Address: "::"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
