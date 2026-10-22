package api

import (
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// TestKickIdempotentUnderConcurrentBroadcasters 回归 P0-0：
// 多个广播 goroutine 同时命中同一慢客户端的满缓冲时，kicked 只能被关闭一次；
// 旧实现是 check-then-close，会 panic: close of closed channel 并打崩进程。
func TestKickIdempotentUnderConcurrentBroadcasters(t *testing.T) {
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := &WSClient{send: make(chan []byte, 1), kicked: make(chan struct{})}
	hub.addClient(c)
	c.send <- []byte("occupied") // 缓冲已满：后续可靠事件必然进入踢除分支

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 16; j++ {
				if j%2 == 0 {
					hub.Broadcast("stream.opened", map[string]interface{}{"i": i})
				} else {
					hub.Broadcast("service.status", map[string]interface{}{"i": i})
				}
			}
		}(i)
	}
	wg.Wait()

	select {
	case <-c.kicked:
	default:
		t.Fatal("slow client should be kicked")
	}

	// kicked 已关闭后继续广播（同一条客户端仍在 hub 中）不应再次 close
	hub.Broadcast("stream.closed", map[string]interface{}{})
	hub.BroadcastDroppable("stream.stats", map[string]interface{}{})
	select {
	case <-c.kicked:
	default:
		t.Fatal("kicked channel should stay closed")
	}
}

// TestKickWithoutKickedChannel 测试路径构造的 WSClient 没有 kicked 时不应 panic
func TestKickWithoutKickedChannel(t *testing.T) {
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := &WSClient{send: make(chan []byte, 1)}
	hub.addClient(c)
	c.send <- []byte("occupied")

	hub.Broadcast("stream.opened", nil)
	hub.Broadcast("stream.opened", nil) // 第二次仍应安全（Once 已消耗）
}

// TestWSMaxClientLimit 验证客户端上限：达到上限后新的升级请求被明确拒绝
func TestWSMaxClientLimit(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	for i := 0; i < wsMaxClients; i++ {
		// 仅占位，nil conn 由 CloseAll 的判空保护
		s.wsHub.addClient(&WSClient{send: make(chan []byte, 1)})
	}
	if !s.wsHub.atCapacity() {
		t.Fatal("hub should report capacity reached")
	}
	if s.wsHub.tryAddClient(&WSClient{send: make(chan []byte, 1)}) {
		t.Fatal("tryAddClient should reject over capacity")
	}

	_, resp, err := websocket.DefaultDialer.Dial("ws://"+s.Addr()+"/api/v1/ws", nil)
	if err == nil {
		t.Fatal("dial over capacity should fail")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("want 503 over capacity, got %d", status)
	}
	resp.Body.Close()
}

// TestBroadcastSkipsMarshalWithoutClients 无客户端时广播不应序列化（性能路径）
func TestBroadcastSkipsMarshalWithoutClients(t *testing.T) {
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	// 不可序列化的 channel 作为 data：若仍执行 Marshal 会报错但不 panic，
	// 这里只验证不产生 panic 且不登记 seq（无客户端时零成本返回）
	hub.Broadcast("stream.opened", make(chan struct{}))
	hub.topicsMu.Lock()
	seq := hub.seqs["stream"]
	hub.topicsMu.Unlock()
	if seq != 0 {
		t.Fatalf("no-client broadcast should not allocate seq, got %d", seq)
	}
}
