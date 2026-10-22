package api

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/utils"
)

// clientProcFixture 带假时钟与假数据源的解析器，避免单测触发真实系统调用
type clientProcFixture struct {
	r          *clientProcResolver
	now        time.Time
	endpoints  map[utils.TCPEndpointKey]int32
	listErr    error
	tableCalls int
	nameCalls  map[int32]int
}

func newClientProcFixture(endpoints map[utils.TCPEndpointKey]int32) *clientProcFixture {
	f := &clientProcFixture{
		now:       time.Now(),
		endpoints: endpoints,
		nameCalls: make(map[int32]int),
	}
	f.r = newClientProcResolver()
	f.r.now = func() time.Time { return f.now }
	f.r.listEndpoints = func() (map[utils.TCPEndpointKey]int32, error) {
		f.tableCalls++
		return f.endpoints, f.listErr
	}
	f.r.procName = func(pid int32) string {
		f.nameCalls[pid]++
		return "chrome"
	}
	f.r.procStartMs = func(pid int32) int64 { return 1700000000000 + int64(pid) }
	return f
}

func (f *clientProcFixture) advance(d time.Duration) { f.now = f.now.Add(d) }

func makeInboundLoopbackStream(remote string) *stream.State {
	return &stream.State{
		Index:      3,
		Direction:  stream.DirectionInbound,
		Created:    time.Now(),
		LocalAddr:  stream.NewAddr("tcp", "127.0.0.1:1080"),
		RemoteAddr: stream.NewAddr("tcp", remote),
	}
}

func TestClientProcResolveInboundLoopback(t *testing.T) {
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{
		{IP: "127.0.0.1", Port: 64443}: 100,
	})
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	var dto StreamStateDTO
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 100 || dto.ClientProcName != "chrome" || dto.ClientProcStartMs != 1700000000100 {
		t.Fatalf("enrich = %+v, want pid=100 name=chrome startMs=1700000000100", dto)
	}
	if f.tableCalls != 1 || f.nameCalls[100] != 1 {
		t.Fatalf("tableCalls=%d nameCalls=%d, want 1/1", f.tableCalls, f.nameCalls[100])
	}

	// 已解析的流走记忆：表缓存过期后再次 enrich 不再触发枚举与进程查询
	f.advance(2 * time.Second)
	dto = StreamStateDTO{}
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 100 || dto.ClientProcName != "chrome" {
		t.Fatalf("memoized enrich = %+v, want pid=100 name=chrome", dto)
	}
	if f.tableCalls != 1 || f.nameCalls[100] != 1 {
		t.Fatalf("after memo hit: tableCalls=%d nameCalls=%d, want 1/1", f.tableCalls, f.nameCalls[100])
	}
}

func TestClientProcSkipNonInboundLoopback(t *testing.T) {
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{
		{IP: "127.0.0.1", Port: 64443}: 100,
	})

	cases := map[string]*stream.State{
		// outbound：本机端点是 agent 自身，无意义
		"outbound": {
			Index:      1,
			Direction:  stream.DirectionOutbound,
			Created:    time.Now(),
			LocalAddr:  stream.NewAddr("tcp", "127.0.0.1:1080"),
			RemoteAddr: stream.NewAddr("tcp", "127.0.0.1:64443"),
		},
		// 非 loopback 远端客户端不解析
		"remote client": makeInboundLoopbackStream("10.0.0.2:5000"),
		// flex 虚拟地址（非 ip:port）无法解析，跳过
		"flex addr": makeInboundLoopbackStream("2:50001"),
		// 无端口文本
		"no port": makeInboundLoopbackStream("127.0.0.1"),
	}
	for name, st := range cases {
		t.Run(name, func(t *testing.T) {
			var dto StreamStateDTO
			f.r.enrich("mynet", st, &dto)
			if dto.ClientPid != 0 || dto.ClientProcName != "" || dto.ClientProcStartMs != 0 {
				t.Fatalf("enrich = %+v, want all zero", dto)
			}
		})
	}
	if f.tableCalls != 0 {
		t.Fatalf("tableCalls = %d, want 0 (no enumeration for skipped streams)", f.tableCalls)
	}
	if len(f.r.entries) != 0 {
		t.Fatalf("entries = %d, want 0 (skipped streams should not be memoized)", len(f.r.entries))
	}
}

func TestClientProcRetryAfterMiss(t *testing.T) {
	// 缓存表早于连接建立（首轮 miss），TTL 过期后重建命中
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{})
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	var dto StreamStateDTO
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 0 {
		t.Fatalf("first enrich should miss, got pid=%d", dto.ClientPid)
	}
	if f.tableCalls != 1 {
		t.Fatalf("tableCalls = %d, want 1", f.tableCalls)
	}

	// TTL 未过期：重试仍查旧表，不重建
	f.r.enrich("mynet", st, &dto)
	if f.tableCalls != 1 {
		t.Fatalf("within TTL: tableCalls = %d, want 1", f.tableCalls)
	}

	// TTL 过期后重试命中
	f.endpoints = map[utils.TCPEndpointKey]int32{{IP: "127.0.0.1", Port: 64443}: 100}
	f.advance(2 * time.Second)
	dto = StreamStateDTO{}
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 100 || dto.ClientProcName != "chrome" {
		t.Fatalf("retry enrich = %+v, want pid=100 name=chrome", dto)
	}
	if f.tableCalls != 2 {
		t.Fatalf("after TTL expiry: tableCalls = %d, want 2", f.tableCalls)
	}
}

func TestClientProcGiveUpAfterWindow(t *testing.T) {
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{})
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	var dto StreamStateDTO
	f.r.enrich("mynet", st, &dto)

	// 重试窗口届满仍未命中：放弃解析，字段留空
	f.advance(clientProcResolveWindow + time.Second)
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 0 || dto.ClientProcName != "" {
		t.Fatalf("after give-up: enrich = %+v, want zero", dto)
	}
	callsAfterGiveUp := f.tableCalls

	// 放弃后不再重试（不再触发表查询/重建）
	f.advance(2 * time.Second)
	f.r.enrich("mynet", st, &dto)
	if f.tableCalls != callsAfterGiveUp {
		t.Fatalf("after give-up: tableCalls = %d, want %d (no retry)", f.tableCalls, callsAfterGiveUp)
	}
}

func TestClientProcEnumFailureDegrades(t *testing.T) {
	f := newClientProcFixture(nil)
	f.listErr = errors.New("permission denied")
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	var dto StreamStateDTO
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 0 || dto.ClientProcName != "" {
		t.Fatalf("enum failure: enrich = %+v, want zero", dto)
	}

	// 枚举失败照常推进 tableAt：TTL 内不再重复触发枚举
	f.r.enrich("mynet", st, &dto)
	if f.tableCalls != 1 {
		t.Fatalf("within TTL after failure: tableCalls = %d, want 1", f.tableCalls)
	}

	// 枚举恢复后重试可命中
	f.listErr = nil
	f.endpoints = map[utils.TCPEndpointKey]int32{{IP: "127.0.0.1", Port: 64443}: 100}
	f.advance(2 * time.Second)
	dto = StreamStateDTO{}
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 100 {
		t.Fatalf("after recovery: enrich pid = %d, want 100", dto.ClientPid)
	}
}

func TestClientProcPidZeroResolvedEmpty(t *testing.T) {
	// 端点归属行存在但 pid 未知（权限不足）：视为终态，不再重试、不查进程名
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{
		{IP: "127.0.0.1", Port: 64443}: 0,
	})
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	var dto StreamStateDTO
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 0 || dto.ClientProcName != "" {
		t.Fatalf("pid=0: enrich = %+v, want zero", dto)
	}
	if len(f.nameCalls) != 0 {
		t.Fatalf("pid=0 should not query process name, got %v", f.nameCalls)
	}

	f.advance(2 * time.Second)
	f.r.enrich("mynet", st, &dto)
	if f.tableCalls != 1 {
		t.Fatalf("pid=0 should be final: tableCalls = %d, want 1", f.tableCalls)
	}
}

func TestClientProcSweep(t *testing.T) {
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{
		{IP: "127.0.0.1", Port: 64443}: 100,
		{IP: "127.0.0.1", Port: 64444}: 101,
	})
	stA := makeInboundLoopbackStream("127.0.0.1:64443")
	stB := makeInboundLoopbackStream("127.0.0.1:64444")
	stB.Index = 4

	var dto StreamStateDTO
	f.r.enrich("mynet", stA, &dto)
	f.r.enrich("mynet", stB, &dto)

	// A 保持活跃（持续被引用），B 闲置
	f.advance(5 * time.Minute)
	f.r.enrich("mynet", stA, &dto)
	f.advance(6 * time.Minute)

	f.r.sweep(f.now)
	if len(f.r.entries) != 1 {
		t.Fatalf("after sweep: entries = %d, want 1", len(f.r.entries))
	}

	// A 未被淘汰仍走记忆；B 被淘汰后重新解析
	f.r.enrich("mynet", stA, &dto)
	if f.tableCalls != 1 {
		t.Fatalf("active entry should survive sweep: tableCalls = %d, want 1", f.tableCalls)
	}
	f.r.enrich("mynet", stB, &dto)
	if dto.ClientPid != 101 {
		t.Fatalf("re-resolved B: pid = %d, want 101", dto.ClientPid)
	}
}

func TestClientProcConcurrent(t *testing.T) {
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{
		{IP: "127.0.0.1", Port: 64443}: 100,
	})
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				var dto StreamStateDTO
				f.r.enrich("mynet", st, &dto)
				f.r.sweep(f.now)
			}
		}()
	}
	wg.Wait()
}
