package api

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTopicOf(t *testing.T) {
	cases := map[string]string{
		"stream.opened":  "stream",
		"network.stats":  "network",
		"hub.snapshot":   "hub",
		"service.status": "service",
		"nosplit":        "nosplit",
	}
	for in, want := range cases {
		if got := topicOf(in); got != want {
			t.Errorf("topicOf(%q): want %q, got %q", in, want, got)
		}
	}
}

// dialTestWS 建立测试 WS 连接并注册清理
func dialTestWS(t *testing.T, s *Server) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/api/v1/ws", s.Addr()), nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readWSMessage 带超时读一条事件
func readWSMessage(t *testing.T, conn *websocket.Conn) Event {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var evt Event
	if err := conn.ReadJSON(&evt); err != nil {
		t.Fatalf("read ws event: %v", err)
	}
	return evt
}

func TestSubscribeTopicsSnapshot(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	conn := dialTestWS(t, s)

	if err := conn.WriteJSON(map[string]interface{}{
		"action": "subscribe",
		"topics": []string{"network"},
	}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	evt := readWSMessage(t, conn)
	if evt.Type != "network.snapshot" {
		t.Fatalf("want network.snapshot, got %q", evt.Type)
	}
	if evt.Topic != "network" {
		t.Errorf("snapshot topic mismatch: %q", evt.Topic)
	}
	// fakeBackend 无网络，快照为空数组而非 null
	if arr, ok := evt.Data.([]interface{}); !ok || len(arr) != 0 {
		t.Errorf("want empty array snapshot, got %v", evt.Data)
	}
}

func TestResyncResendsSnapshot(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	conn := dialTestWS(t, s)

	conn.WriteJSON(map[string]interface{}{"action": "subscribe", "topics": []string{"hub"}})
	evt := readWSMessage(t, conn)
	if evt.Type != "hub.snapshot" {
		t.Fatalf("want hub.snapshot, got %q", evt.Type)
	}
	data := evt.Data.(map[string]interface{})
	if data["startedAtMs"].(float64) <= 0 {
		t.Errorf("hub.snapshot should carry startedAtMs > 0, got %v", data["startedAtMs"])
	}
	if data["protocol"].(float64) != float64(testMeta.Protocol) {
		t.Errorf("protocol mismatch: %v", data["protocol"])
	}

	// resync 重发快照（无论是否订阅）
	conn.WriteJSON(map[string]interface{}{"action": "resync", "topics": []string{"hub", "stream"}})
	got := map[string]bool{}
	for i := 0; i < 2; i++ {
		evt := readWSMessage(t, conn)
		got[evt.Type] = true
	}
	if !got["hub.snapshot"] || !got["stream.snapshot"] {
		t.Fatalf("resync should resend both snapshots, got %v", got)
	}

	// 未知主题：不回应
	conn.WriteJSON(map[string]interface{}{"action": "resync", "topics": []string{"nope"}})
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var evt2 Event
	if err := conn.ReadJSON(&evt2); err == nil {
		t.Fatalf("unknown topic should not produce snapshot, got %+v", evt2)
	}
}

func TestTopicFilterExcludesOtherTopics(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	conn := dialTestWS(t, s)

	// 只订阅 network 主题：service 主题事件不应送达
	conn.WriteJSON(map[string]interface{}{"action": "subscribe", "topics": []string{"network"}})
	readWSMessage(t, conn) // network.snapshot

	s.wsHub.Broadcast("network.state", map[string]interface{}{"name": "mynet"})
	s.wsHub.Broadcast("service.status", map[string]interface{}{"name": "svc"})

	evt := readWSMessage(t, conn)
	if evt.Type != "network.state" {
		t.Fatalf("want network.state, got %q", evt.Type)
	}

	// 空数组 = 退订全部（无快照回执，稍等订阅生效）
	conn.WriteJSON(map[string]interface{}{"action": "subscribe", "topics": []string{}})
	time.Sleep(100 * time.Millisecond)
	s.wsHub.Broadcast("network.state", map[string]interface{}{"name": "mynet"})
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var evt2 Event
	if err := conn.ReadJSON(&evt2); err == nil {
		t.Fatalf("unsubscribed client should receive nothing, got %+v", evt2)
	}
}

func TestLegacyEventsFilterStillWorks(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	conn := dialTestWS(t, s)

	// v1 事件类型过滤：不触发快照，按事件类型精确匹配（无回执，稍等订阅生效）
	conn.WriteJSON(map[string]interface{}{"action": "subscribe", "events": []string{"network.state"}})
	time.Sleep(100 * time.Millisecond)
	s.wsHub.Broadcast("network.state", map[string]interface{}{"name": "mynet"})
	s.wsHub.Broadcast("network.stats", map[string]interface{}{"networks": []interface{}{}})

	evt := readWSMessage(t, conn)
	if evt.Type != "network.state" {
		t.Fatalf("want network.state, got %q", evt.Type)
	}
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var evt2 Event
	if err := conn.ReadJSON(&evt2); err == nil {
		t.Fatalf("events filter should exclude network.stats, got %+v", evt2)
	}
}

func TestSeqMonotonicPerTopic(t *testing.T) {
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := &WSClient{send: make(chan []byte, 64)}
	hub.addClient(c)

	hub.Broadcast("network.state", nil)
	hub.Broadcast("network.stats", nil)
	hub.Broadcast("service.status", nil)

	var lastSeq int64
	for i := 0; i < 2; i++ {
		evt := readWSEvent(t, c)
		if evt.Topic != "network" {
			t.Fatalf("want network topic, got %q", evt.Topic)
		}
		if evt.Seq <= lastSeq {
			t.Fatalf("seq should increase within topic, got %d after %d", evt.Seq, lastSeq)
		}
		lastSeq = evt.Seq
	}
	evt := readWSEvent(t, c)
	if evt.Topic != "service" || evt.Seq != 1 {
		t.Errorf("service topic seq should start at 1, got topic=%q seq=%d", evt.Topic, evt.Seq)
	}
}

func TestSlowClientPolicy(t *testing.T) {
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// 可靠事件：缓冲区满 → 踢掉（kicked 关闭）
	c1 := &WSClient{send: make(chan []byte, 1), kicked: make(chan struct{})}
	hub.addClient(c1)
	c1.send <- []byte("occupied")
	hub.Broadcast("stream.opened", map[string]interface{}{})
	select {
	case <-c1.kicked:
	default:
		t.Fatal("reliable event overflow should kick the client")
	}

	// 可丢弃事件：缓冲区满 → 跳过，不踢
	c2 := &WSClient{send: make(chan []byte, 1), kicked: make(chan struct{})}
	hub.addClient(c2)
	c2.send <- []byte("occupied")
	hub.BroadcastDroppable("stream.stats", map[string]interface{}{})
	select {
	case <-c2.kicked:
		t.Fatal("droppable event overflow should not kick the client")
	default:
	}
}

func TestStreamEventBatcher(t *testing.T) {
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := &WSClient{send: make(chan []byte, 64)}
	hub.addClient(c)

	batcher := newStreamEventBatcher()
	batcher.add(StreamLifecycleEvent{Network: "mynet", State: makeTestStreamState()})
	batcher.add(StreamLifecycleEvent{Network: "mynet", State: makeTestStreamState()})

	closed := makeTestStreamState()
	closed.IsClosed = true
	closed.Closed = time.Now()
	batcher.add(StreamLifecycleEvent{Network: "mynet", State: closed, Closed: true})

	batcher.flush(hub, func(network string, index int64) string { return "svc" })

	evt := readWSEvent(t, c)
	if evt.Type != "stream.opened" {
		t.Fatalf("want stream.opened, got %q", evt.Type)
	}
	data := evt.Data.(map[string]interface{})
	if data["network"] != "mynet" {
		t.Errorf("batch network mismatch: %v", data["network"])
	}
	streams := data["streams"].([]interface{})
	if len(streams) != 2 {
		t.Fatalf("want 2 opened streams in one batch, got %d", len(streams))
	}
	if streams[0].(map[string]interface{})["serviceName"] != "svc" {
		t.Errorf("opened streams should be enriched with serviceName: %v", streams[0])
	}

	evt = readWSEvent(t, c)
	if evt.Type != "stream.closed" {
		t.Fatalf("want stream.closed, got %q", evt.Type)
	}
	closedStreams := evt.Data.(map[string]interface{})["streams"].([]interface{})
	if len(closedStreams) != 1 {
		t.Fatalf("want 1 closed stream, got %d", len(closedStreams))
	}
	dto := closedStreams[0].(map[string]interface{})
	if dto["closedAtMs"].(float64) <= 0 {
		t.Errorf("closed stream should carry closedAtMs, got %v", dto["closedAtMs"])
	}

	// 冲刷后批处理器应为空
	select {
	case msg := <-c.send:
		t.Fatalf("unexpected extra event after flush: %s", msg)
	default:
	}
}

// TestWsHeartbeatConfig 验证握手后服务端配置了读超时与 ping（协议层心跳存在性检查：
// 连接后 30s 周期的 ping 在测试中不等待，仅验证服务端 writePump/readPump 正常工作）
func TestWsConnectionSurvivesIdle(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})
	conn := dialTestWS(t, s)

	// 空闲 300ms 后连接应仍可用（服务端主动推送可达）
	time.Sleep(300 * time.Millisecond)
	s.wsHub.Broadcast("network.state", map[string]interface{}{"name": "mynet"})
	evt := readWSMessage(t, conn)
	if evt.Type != "network.state" {
		t.Fatalf("want network.state, got %q", evt.Type)
	}
}
