package utils

import (
	"testing"
)

// netstat -anv -p tcp 真实输出样式（macOS）：进程名截断且可能含空格，
// 覆盖 ESTABLISHED / TIME_WAIT / LISTEN / IPv6 / 通配监听行（tcp4/tcp6/tcp46）
const netstatTCPFixture = `Active Internet connections (including servers)
Proto Recv-Q Send-Q  Local Address                                 Foreign Address                               (state)          rxbytes      txbytes  rhiwat  shiwat          process:pid    state  options           gencnt    flags   flags1 usecnt rtncnt fltrs
tcp4       0      0  127.0.0.1.21080        127.0.0.1.58511        ESTABLISHED         9894       156378  403968  146988       remotework:88086  00102 0000000c 000000000235ddf1 00000080 01000800      2      0 000000
tcp4       0      0  127.0.0.1.58511        127.0.0.1.21080        ESTABLISHED       314628         4609  513869  146988     verge-mihomo:33174  00182 00000008 000000000235ddf0 00000080 04000900      2      0 000000
tcp4       0      0  127.0.0.1.58510        127.0.0.1.10807        ESTABLISHED       314628         4609  513869  146988 Google Chrome He:24954  00102 00000008 000000000235dded 00000080 04000900      2      0 000000
tcp4       0      0  127.0.0.1.58558        127.0.0.1.58557        TIME_WAIT             52            0  408300  146988       python3.13:9070   02031 00000000 000000000235e347 00000080 04000900      0      0 000000
tcp6       0      0  ::1.5000               ::1.1080               ESTABLISHED            0            0  408300  146988           kimi:56272      00102 00000008 000000000235dd66 00000001 04000900      2      0 000000
tcp4       0      0  *.1080                 *.*                    LISTEN                 0            0  408300       0       remotework:88086  00102 00000008 000000000235dd65 00000080 04000900      0      0 000000
tcp6       0      0  *.49870                *.*                    LISTEN                 0            0  131072  131072         rapportd:621    00100 00000006 000000000233e287 00000001 00000800      1      0 000000
tcp46      0      0  *.51372                *.*                    LISTEN                 0            0  131072  131072      wpscloudsvr:33506  00100 00000106 0000000002238599 00000000 00000800      1      0 000000
`

func TestParseNetstatTCP(t *testing.T) {
	got := buildTCPLocalEndpoints(parseNetstatTCP([]byte(netstatTCPFixture)))
	want := map[TCPEndpointKey]int32{
		{IP: "127.0.0.1", Port: 21080}: 88086, // 服务端行
		{IP: "127.0.0.1", Port: 58511}: 33174, // root 进程（lsof 盲区）经 netstat 可见
		{IP: "127.0.0.1", Port: 58510}: 24954, // 进程名含空格（截断展示）
		{IP: "127.0.0.1", Port: 58558}: 9070,  // TIME_WAIT 残留仍带属主（短连接补解析的基础）
		{IP: "::1", Port: 5000}:        56272, // IPv6
		{IP: "0.0.0.0", Port: 1080}:    88086, // tcp4 通配监听
		{IP: "::", Port: 49870}:        621,   // tcp6 通配监听
		{IP: "::", Port: 51372}:        33506, // tcp46 双栈通配监听
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for key, pid := range want {
		if got[key] != pid {
			t.Errorf("got[%+v] = %d, want %d", key, got[key], pid)
		}
	}
}

func TestParseNetstatTCPAddr(t *testing.T) {
	cases := []struct {
		in    string
		proto string
		ip    string
		port  uint32
		ok    bool
	}{
		{"127.0.0.1.21080", "tcp4", "127.0.0.1", 21080, true},
		{"::1.8080", "tcp6", "::1", 8080, true},
		{"::1%lo0.8080", "tcp6", "::1", 8080, true}, // 剥去 IPv6 zone
		{"*.1080", "tcp4", "0.0.0.0", 1080, true},   // tcp4 通配监听
		{"*.49870", "tcp6", "::", 49870, true},      // tcp6 通配监听
		{"*.51372", "tcp46", "::", 51372, true},     // tcp46 双栈通配监听
		{"127.0.0.1.0", "tcp4", "", 0, false},       // 端口非法
		{"127.0.0.1.65536", "tcp4", "", 0, false},
		{"garbage", "tcp4", "", 0, false},
		{"*.*", "tcp4", "", 0, false}, // 对端通配（外部地址列形态），无端口不可用
	}
	for _, c := range cases {
		ip, port, ok := parseNetstatTCPAddr(c.in, c.proto)
		if ok != c.ok || (ok && (ip != c.ip || port != c.port)) {
			t.Errorf("parseNetstatTCPAddr(%q, %q) = (%q, %d, %v), want (%q, %d, %v)",
				c.in, c.proto, ip, port, ok, c.ip, c.port, c.ok)
		}
	}
}

// 监听枚举视角：netstat 解析行直接喂给 buildListeningPorts，
// 通配监听行应映射为文档约定的 0.0.0.0 / :: 地址，协议名应为 tcp
func TestParseNetstatTCPForListeningPorts(t *testing.T) {
	origin := getProcessStartMs
	getProcessStartMs = func(pid int32) int64 { return 1700000000000 + int64(pid) }
	t.Cleanup(func() { getProcessStartMs = origin })

	items := buildListeningPorts(parseNetstatTCP([]byte(netstatTCPFixture)))
	want := []ListeningPortInfo{
		{Port: 1080, Protocol: "tcp", PID: 88086, Address: "0.0.0.0"},
		{Port: 49870, Protocol: "tcp", PID: 621, Address: "::"},
		{Port: 51372, Protocol: "tcp", PID: 33506, Address: "::"},
	}
	if len(items) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(items), len(want), items)
	}
	for i, w := range want {
		got := items[i]
		if got.Port != w.Port || got.Protocol != w.Protocol || got.PID != w.PID || got.Address != w.Address {
			t.Errorf("items[%d] = {Port: %d, Protocol: %q, PID: %d, Address: %q}, want %+v",
				i, got.Port, got.Protocol, got.PID, got.Address, w)
		}
		if got.ProcStartMs != 1700000000000+int64(got.PID) {
			t.Errorf("items[%d].ProcStartMs = %d, want %d", i, got.ProcStartMs, 1700000000000+int64(got.PID))
		}
	}
}
