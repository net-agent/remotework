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
	}

	got := buildListeningPorts(connections)
	if len(got) != 2 {
		t.Fatalf("len(buildListeningPorts()) = %d, want 2", len(got))
	}
	if got[0].Port != 22 {
		t.Fatalf("first port = %d, want 22", got[0].Port)
	}
	if got[1].Port != 8080 {
		t.Fatalf("second port = %d, want 8080", got[1].Port)
	}
}

func TestDedupeListeningPorts(t *testing.T) {
	items := []ListeningPortInfo{
		{Port: 8080, Protocol: "tcp", PID: 1, ProcessName: "a"},
		{Port: 22, Protocol: "tcp", PID: 2, ProcessName: "ssh"},
		{Port: 8080, Protocol: "tcp", PID: 3, ProcessName: "b"},
	}

	got := dedupeListeningPorts(items)
	if len(got) != 2 {
		t.Fatalf("len(dedupeListeningPorts()) = %d, want 2", len(got))
	}
	if got[0].Port != 22 || got[1].Port != 8080 {
		t.Fatalf("ports = [%d, %d], want [22, 8080]", got[0].Port, got[1].Port)
	}
	if got[1].PID != 1 {
		t.Fatalf("dedupe should keep first occurrence PID=1, got %d", got[1].PID)
	}
}
