package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// annotatorBackend 实现 StreamAnnotator 的流后端：统计批量标注调用与回退调用
type annotatorBackend struct {
	streamBackend
	annotateCalls   int
	annotateIndexes []int64
	annotations     map[int64]StreamAnnotation
	fallbackCalls   int
}

func (b *annotatorBackend) GetAllNetworkState() ([]vnet.NetworkReport, error) {
	return []vnet.NetworkReport{{Name: "mynet"}}, nil
}

func (b *annotatorBackend) AnnotateStreams(network string, indexes []int64) map[int64]StreamAnnotation {
	b.annotateCalls++
	b.annotateIndexes = append(b.annotateIndexes, indexes...)
	return b.annotations
}

func (b *annotatorBackend) GetStreamService(network string, index int64) string {
	b.fallbackCalls++
	return "fallback"
}

func (b *annotatorBackend) GetStreamPipe(network string, index int64) string {
	b.fallbackCalls++
	return "fallback-pipe"
}

// TestPollerUsesBatchAnnotator poller 每条流不再各调一次 GetStreamService/GetStreamPipe
func TestPollerUsesBatchAnnotator(t *testing.T) {
	st1 := makeTestStreamState() // index=7
	st2 := makeTestStreamState()
	st2.Index = 8

	b := &annotatorBackend{
		streamBackend: streamBackend{streams: []*DataStreamState{{
			Network: "mynet",
			Actives: []*stream.State{st1, st2},
		}}},
		annotations: map[int64]StreamAnnotation{
			7: {ServiceName: "rdp", PipeID: "rdp#1"},
			8: {ServiceName: "ssh", PipeID: "ssh#2"},
		},
	}

	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	poller := NewStatePoller(b, hub, nil, nil, time.Hour, time.Hour)
	c := &WSClient{send: make(chan []byte, 16)}
	hub.addClient(c)

	poller.pollStreamStats()

	if b.annotateCalls != 1 {
		t.Fatalf("AnnotateStreams calls = %d, want 1", b.annotateCalls)
	}
	if len(b.annotateIndexes) != 2 {
		t.Fatalf("annotate indexes = %v, want both streams", b.annotateIndexes)
	}
	if b.fallbackCalls != 0 {
		t.Fatalf("fallback GetStreamService should not be called, got %d", b.fallbackCalls)
	}

	events := collectWSEvents(t, c, 2)
	stats, ok := events["stream.stats"]
	if !ok {
		t.Fatalf("want stream.stats, got %v", events)
	}
	streams := stats.Data.(map[string]interface{})["streams"].([]interface{})
	if len(streams) != 2 {
		t.Fatalf("want 2 streams in stats, got %d", len(streams))
	}
	byIndex := map[float64]map[string]interface{}{}
	for _, item := range streams {
		m := item.(map[string]interface{})
		byIndex[m["index"].(float64)] = m
	}
	if got := byIndex[7]["serviceName"]; got != "rdp" {
		t.Errorf("stream 7 serviceName = %v, want rdp", got)
	}
	if got := byIndex[8]["pipeId"]; got != "ssh#2" {
		t.Errorf("stream 8 pipeId = %v, want ssh#2", got)
	}
}

// TestHandleStreamsUsesBatchAnnotator REST /streams 同样走批量标注（每网络一次）
func TestHandleStreamsUsesBatchAnnotator(t *testing.T) {
	st := makeTestStreamState()
	closed := makeTestStreamState()
	closed.Index = 9
	closed.IsClosed = true
	closed.Closed = time.Now()

	b := &annotatorBackend{
		streamBackend: streamBackend{streams: []*DataStreamState{{
			Network: "mynet",
			Actives: []*stream.State{st},
			Closeds: []*stream.State{closed},
		}}},
		annotations: map[int64]StreamAnnotation{
			7: {ServiceName: "rdp", PipeID: "rdp#1"},
			9: {ServiceName: "ssh", PipeID: "ssh#2"},
		},
	}
	s := startTestServerWith(t, b, Config{Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/streams", "", nil)
	env := decodeEnvelope(t, resp)
	if env.ErrCode != 0 {
		t.Fatalf("streams: %s", env.ErrMsg)
	}
	var dtos []StreamStateDTO
	if err := json.Unmarshal(env.Data, &dtos); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(dtos) != 2 {
		t.Fatalf("want 2 streams, got %d", len(dtos))
	}
	if b.annotateCalls != 1 {
		t.Fatalf("AnnotateStreams calls = %d, want 1 (one batch per network)", b.annotateCalls)
	}
	if b.fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0", b.fallbackCalls)
	}
}

// fallbackAnnotBackend 未实现 StreamAnnotator：annotateStreams 必须回退逐流查询
type fallbackAnnotBackend struct {
	fakeBackend
}

func (b *fallbackAnnotBackend) GetStreamService(network string, index int64) string {
	return "svc"
}

func (b *fallbackAnnotBackend) GetStreamPipe(network string, index int64) string {
	return "pipe"
}

func TestAnnotateStreamsFallback(t *testing.T) {
	out := annotateStreams(&fallbackAnnotBackend{}, "mynet", []int64{1, 2})
	if len(out) != 2 {
		t.Fatalf("got %d annotations, want 2", len(out))
	}
	for _, index := range []int64{1, 2} {
		if out[index].ServiceName != "svc" || out[index].PipeID != "pipe" {
			t.Errorf("annotation[%d] = %+v", index, out[index])
		}
	}
	if got := annotateStreams(&fallbackAnnotBackend{}, "mynet", nil); got != nil {
		t.Errorf("empty indexes should return nil, got %v", got)
	}
}

// TestPollServicesIgnoreCounters service.status 差分不看 actives/dones（计数走 stats 通道）
func TestPollServicesIgnoreCounters(t *testing.T) {
	svc := &vservice.ServiceState{Name: "rdp"}
	svc.SetStatus(vservice.StatusRunning)
	b := &serviceBackend{states: []*vservice.ServiceState{svc}}

	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	poller := NewStatePoller(b, hub, nil, nil, time.Hour, time.Hour)
	c := &WSClient{send: make(chan []byte, 16)}
	hub.addClient(c)

	// 首轮：新服务 → 一条可靠事件
	poller.pollServices()
	evt := readWSEvent(t, c)
	if evt.Type != "service.status" {
		t.Fatalf("want service.status, got %q", evt.Type)
	}

	// 仅计数变化 → 不应广播（旧实现会把每条连接完成推成可靠事件）
	svc.AddActiveCount(1)
	svc.AddDoneCount(1)
	poller.pollServices()
	select {
	case msg := <-c.send:
		t.Fatalf("counter-only change should not broadcast, got %s", msg)
	default:
	}

	// 状态变化 → 广播
	svc.SetStatus(vservice.StatusStopped)
	poller.pollServices()
	evt = readWSEvent(t, c)
	if evt.Type != "service.status" {
		t.Fatalf("status change should broadcast, got %q", evt.Type)
	}

	// LastErr 变化 → 广播
	svc.SetLastErr(errors.New("dial failed"))
	poller.pollServices()
	evt = readWSEvent(t, c)
	if evt.Type != "service.status" {
		t.Fatalf("lastErr change should broadcast, got %q", evt.Type)
	}
}

// TestPollStreamStatsNilStates Actives/Closeds 含 nil 时不得 panic（P2）
func TestPollStreamStatsNilStates(t *testing.T) {
	st := makeTestStreamState()
	closed := makeTestStreamState()
	closed.Index = 8
	closed.IsClosed = true
	closed.Closed = time.Now()

	b := &streamBackend{streams: []*DataStreamState{{
		Network: "mynet",
		Actives: []*stream.State{nil, st},
		Closeds: []*stream.State{nil, closed},
	}}}
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	poller := NewStatePoller(b, hub, nil, nil, time.Hour, time.Hour)
	c := &WSClient{send: make(chan []byte, 16)}
	hub.addClient(c)

	poller.pollStreamStats() // 不应 panic

	events := collectWSEvents(t, c, 2)
	stats, ok := events["stream.stats"]
	if !ok {
		t.Fatalf("want stream.stats, got %v", events)
	}
	streams := stats.Data.(map[string]interface{})["streams"].([]interface{})
	if len(streams) != 1 {
		t.Fatalf("nil states should be skipped, got %d streams", len(streams))
	}
}

// TestPollerCleansOfflineBaselines 网络/服务下线后清理差分基线，防泄漏与误比对
func TestPollerCleansOfflineBaselines(t *testing.T) {
	b := &streamBackend{}
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	poller := NewStatePoller(b, hub, nil, nil, time.Hour, time.Hour)

	poller.lastStreams["gone"] = map[string]bool{"a": true}
	poller.lastStreamStats["gone"] = map[int32]StreamStateDTO{1: {}}
	poller.streamAnchorAt["gone"] = time.Now()
	poller.lastSvcStates["gone-svc"] = svcSnapshot{status: "running"}

	poller.pollStreamStats()
	poller.pollServices()

	if _, ok := poller.lastStreams["gone"]; ok {
		t.Error("lastStreams for offline network not cleaned")
	}
	if _, ok := poller.lastStreamStats["gone"]; ok {
		t.Error("lastStreamStats for offline network not cleaned")
	}
	if _, ok := poller.streamAnchorAt["gone"]; ok {
		t.Error("streamAnchorAt for offline network not cleaned")
	}
	if _, ok := poller.lastSvcStates["gone-svc"]; ok {
		t.Error("lastSvcStates for removed service not cleaned")
	}
}
