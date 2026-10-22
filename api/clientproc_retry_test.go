package api

import (
	"testing"
	"time"

	"github.com/net-agent/remotework/utils"
)

// TestClientProcRetryPendingClosedStream 短连接场景：开流首次查表 miss 后连接秒关，
// 不再出现在 stats 通道（不再被 enrich 引用）；retryPending 在窗口期内补查命中
// （TIME_WAIT 残留仍带属主），窗口届满则放弃且不再触发表查询
func TestClientProcRetryPendingClosedStream(t *testing.T) {
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{})
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	// 开流 miss（缓存表早于连接建立）
	var dto StreamStateDTO
	f.r.enrich("mynet", st, &dto)
	if dto.ClientPid != 0 {
		t.Fatalf("first enrich should miss, got pid=%d", dto.ClientPid)
	}
	if f.tableCalls != 1 {
		t.Fatalf("tableCalls = %d, want 1", f.tableCalls)
	}

	// 流关闭：不再被 enrich。表 TTL 过期后重建（此时 TIME_WAIT 残留可见），retryPending 命中
	f.endpoints = map[utils.TCPEndpointKey]int32{{IP: "127.0.0.1", Port: 64443}: 100}
	f.advance(2 * time.Second)
	f.r.retryPending()

	dto = StreamStateDTO{}
	f.r.enrich("mynet", st, &dto) // 关闭后的 REST/事件透传走记忆
	if dto.ClientPid != 100 || dto.ClientProcName != "chrome" || dto.ClientProcStartMs != 1700000000100 {
		t.Fatalf("retryPending should resolve closed stream, got %+v", dto)
	}
	if f.tableCalls != 2 {
		t.Fatalf("after retryPending: tableCalls = %d, want 2", f.tableCalls)
	}
}

// TestClientProcRetryPendingGivesUp 窗口届满仍未命中：retryPending 放弃解析，
// 此后不再触发表重建
func TestClientProcRetryPendingGivesUp(t *testing.T) {
	f := newClientProcFixture(map[utils.TCPEndpointKey]int32{})
	st := makeInboundLoopbackStream("127.0.0.1:64443")

	var dto StreamStateDTO
	f.r.enrich("mynet", st, &dto)

	// 窗口期内每轮 retryPending 都会尝试（TTL 过期时重建表）
	f.advance(2 * time.Second)
	f.r.retryPending()
	if f.tableCalls != 2 {
		t.Fatalf("within window: tableCalls = %d, want 2", f.tableCalls)
	}

	// 窗口届满：放弃，标记终态
	f.advance(clientProcResolveWindow)
	f.r.retryPending()
	var dtoAfter StreamStateDTO
	f.r.enrich("mynet", st, &dtoAfter)
	if dtoAfter.ClientPid != 0 {
		t.Fatalf("after give-up: pid = %d, want 0", dtoAfter.ClientPid)
	}
	calls := f.tableCalls

	// 终态后 retryPending 不再查表
	f.advance(2 * time.Second)
	f.r.retryPending()
	if f.tableCalls != calls {
		t.Fatalf("after give-up: tableCalls = %d, want %d (no retry)", f.tableCalls, calls)
	}
}
