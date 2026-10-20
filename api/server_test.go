package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// fakeBackend 空实现的控制面后端，仅用于服务器层测试
type fakeBackend struct{}

func (fakeBackend) IsRunning() bool                                   { return true }
func (fakeBackend) GetAllNetworkState() ([]vnet.NetworkReport, error) { return nil, nil }
func (fakeBackend) GetAllServiceState() ([]*vservice.ServiceState, error) {
	return nil, nil
}
func (fakeBackend) GetDataStreamState(limit int, networks ...string) []*DataStreamState {
	return nil
}
func (fakeBackend) GetPingState() ([]*vnet.PingReport, error) { return nil, nil }
func (fakeBackend) PingDomain(network, domain string) (time.Duration, error) {
	return 0, nil
}
func (fakeBackend) CloseStream(network string, index int32) error        { return nil }
func (fakeBackend) GetStreamService(network string, index int64) string  { return "" }
func (fakeBackend) GetStreamPipe(network string, index int64) string     { return "" }
func (fakeBackend) MountLink(alias string, link configv2.LinkSpec) error { return nil }
func (fakeBackend) MountTunnel(tunnel configv2.TunnelSpec) error         { return nil }
func (fakeBackend) RequestStop()                                         {}
func (fakeBackend) SubscribeNetworkState(buf int) (<-chan NetworkStateEvent, func()) {
	return make(chan NetworkStateEvent), func() {}
}
func (fakeBackend) SubscribeServiceStatus(buf int) (<-chan ServiceStatusEvent, func()) {
	return make(chan ServiceStatusEvent), func() {}
}
func (fakeBackend) SubscribeStreamEvents(buf int) (<-chan StreamLifecycleEvent, func()) {
	return make(chan StreamLifecycleEvent), func() {}
}

var testMeta = Meta{
	Protocol:   1,
	Version:    "1.2.3-test",
	Commit:     "abc1234",
	Mode:       "normal",
	ConfigPath: "/tmp/test-config.json",
}

func startTestServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:0"
	}
	s := New(fakeBackend{}, cfg, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(s.Stop)
	if s.Addr() == "" {
		t.Fatal("Addr() empty after Start")
	}
	return s
}

func doRequest(t *testing.T, method, url, token string, mutate func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if mutate != nil {
		mutate(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func TestStartSyncBindFailure(t *testing.T) {
	// 先占用一个端口，再让服务器绑定同一地址，必须同步报错
	occupied := startTestServer(t, Config{Meta: testMeta})

	s := New(fakeBackend{}, Config{Listen: occupied.Addr(), Meta: testMeta}, nil)
	if err := s.Start(); err == nil {
		s.Stop()
		t.Fatal("expected bind error, got nil")
	}
}

func TestAuthToken(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	// 无 token → 401
	resp := doRequest(t, "GET", base+"/api/v1/status", "", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", resp.StatusCode)
	}

	// 错 token → 401
	resp = doRequest(t, "GET", base+"/api/v1/status", "wrong", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token: want 401, got %d", resp.StatusCode)
	}

	// 正确 token → 200
	resp = doRequest(t, "GET", base+"/api/v1/status", "t0ken", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("correct token: want 200, got %d", resp.StatusCode)
	}
}

func TestHostCheck(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	resp := doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Host = "evil.com"
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("evil host: want 403, got %d", resp.StatusCode)
	}

	resp = doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Host = "127.0.0.1"
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("localhost host: want 200, got %d", resp.StatusCode)
	}
}

func TestCORSAllowList(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	base := "http://" + s.Addr()

	// 白名单命中 → 反射 Origin
	resp := doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Header.Set("Origin", "http://localhost:1420")
	})
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:1420" {
		t.Fatalf("allowed origin: want reflected ACAO, got %q", got)
	}

	// 未命中 → 不设 ACAO
	resp = doRequest(t, "GET", base+"/api/v1/status", "t0ken", func(r *http.Request) {
		r.Header.Set("Origin", "http://evil.example")
	})
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin: want no ACAO, got %q", got)
	}

	// 预检 → 204
	resp = doRequest(t, "OPTIONS", base+"/api/v1/status", "", func(r *http.Request) {
		r.Header.Set("Origin", "tauri://localhost")
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight: want 204, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "tauri://localhost" {
		t.Fatalf("preflight: want reflected ACAO, got %q", got)
	}
}

func TestStatusMeta(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/status", "t0ken", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var envelope struct {
		ErrCode int `json:"ErrCode"`
		Data    struct {
			Running    bool   `json:"running"`
			Mode       string `json:"mode"`
			Version    string `json:"version"`
			Commit     string `json:"commit"`
			Protocol   int    `json:"protocol"`
			UptimeMs   int64  `json:"uptimeMs"`
			ConfigPath string `json:"configPath"`
		} `json:"Data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	d := envelope.Data
	if !d.Running {
		t.Error("running should be true")
	}
	if d.Mode != testMeta.Mode || d.Version != testMeta.Version ||
		d.Commit != testMeta.Commit || d.Protocol != testMeta.Protocol ||
		d.ConfigPath != testMeta.ConfigPath {
		t.Errorf("meta mismatch: %+v", d)
	}
	if d.UptimeMs < 0 {
		t.Errorf("uptimeMs should be >= 0, got %d", d.UptimeMs)
	}
}

func TestStandaloneNoToken(t *testing.T) {
	s := startTestServer(t, Config{Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/status", "", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("standalone without token: want 200, got %d", resp.StatusCode)
	}
}

func TestWebSocketAuth(t *testing.T) {
	s := startTestServer(t, Config{Token: "t0ken", Meta: testMeta})
	wsBase := fmt.Sprintf("ws://%s/api/v1/ws", s.Addr())

	// 无 token → 拒绝握手（401）
	_, resp, err := websocket.DefaultDialer.Dial(wsBase, nil)
	if err == nil {
		t.Fatal("ws without token: want handshake failure")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ws without token: want 401, got %v", resp.StatusCode)
	}
	resp.Body.Close()

	// token 正确但 Origin 非法 → 拒绝握手（403）
	header := http.Header{"Origin": []string{"http://evil.example"}}
	_, resp, err = websocket.DefaultDialer.Dial(wsBase+"?token=t0ken", header)
	if err == nil {
		t.Fatal("ws with bad origin: want handshake failure")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("ws with bad origin: want 403, got %v", resp.StatusCode)
	}
	resp.Body.Close()

	// token 正确且 Origin 合法 → 握手成功
	header = http.Header{"Origin": []string{"http://localhost:1420"}}
	conn, resp, err := websocket.DefaultDialer.Dial(wsBase+"?token=t0ken", header)
	if err != nil {
		t.Fatalf("ws with token: %v", err)
	}
	resp.Body.Close()
	conn.Close()
}

// streamBackend 嵌入 fakeBackend，覆盖流相关方法以支撑流管理端点测试
type streamBackend struct {
	fakeBackend
	streams     []*DataStreamState
	closeErr    error
	serviceName string
	pipeID      string
	closeCalls  []string // 记录 "network/index" 调用
}

func (b *streamBackend) GetDataStreamState(limit int, networks ...string) []*DataStreamState {
	return b.streams
}
func (b *streamBackend) CloseStream(network string, index int32) error {
	b.closeCalls = append(b.closeCalls, fmt.Sprintf("%s/%d", network, index))
	return b.closeErr
}
func (b *streamBackend) GetStreamService(network string, index int64) string {
	return b.serviceName
}
func (b *streamBackend) GetStreamPipe(network string, index int64) string {
	return b.pipeID
}

func startTestServerWith(t *testing.T, b Backend, cfg Config) *Server {
	t.Helper()
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:0"
	}
	s := New(b, cfg, nil)
	if err := s.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(s.Stop)
	return s
}

// apiEnvelope 统一响应信封
type apiEnvelope struct {
	ErrCode int             `json:"ErrCode"`
	ErrMsg  string          `json:"ErrMsg"`
	Data    json.RawMessage `json:"Data"`
}

func decodeEnvelope(t *testing.T, resp *http.Response) apiEnvelope {
	t.Helper()
	defer resp.Body.Close()
	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return env
}

func TestCloseStreamEndpoint(t *testing.T) {
	b := &streamBackend{}
	s := startTestServerWith(t, b, Config{Meta: testMeta})
	base := "http://" + s.Addr()

	// 成功关闭
	resp := doRequest(t, "DELETE", base+"/api/v1/streams/mynet/7", "", nil)
	env := decodeEnvelope(t, resp)
	if env.ErrCode != 0 {
		t.Fatalf("close stream: want ErrCode=0, got %d (%s)", env.ErrCode, env.ErrMsg)
	}
	var data map[string]string
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data["status"] != "ok" {
		t.Fatalf("want status=ok, got %v", data)
	}
	if len(b.closeCalls) != 1 || b.closeCalls[0] != "mynet/7" {
		t.Fatalf("closeCalls mismatch: %v", b.closeCalls)
	}

	// 流不存在
	b.closeErr = errors.New("stream='mynet/99' not found")
	resp = doRequest(t, "DELETE", base+"/api/v1/streams/mynet/99", "", nil)
	env = decodeEnvelope(t, resp)
	if env.ErrCode == 0 {
		t.Fatal("stream not found: want ErrCode!=0")
	}
	b.closeErr = nil

	// 网络不支持关闭
	b.closeErr = errors.New("network='tcp' does not support closing streams")
	resp = doRequest(t, "DELETE", base+"/api/v1/streams/tcp/1", "", nil)
	env = decodeEnvelope(t, resp)
	if env.ErrCode == 0 {
		t.Fatal("close unsupported: want ErrCode!=0")
	}
	b.closeErr = nil

	// 非法 id
	resp = doRequest(t, "DELETE", base+"/api/v1/streams/mynet/abc", "", nil)
	env = decodeEnvelope(t, resp)
	if env.ErrCode == 0 {
		t.Fatal("invalid id: want ErrCode!=0")
	}
}

// makeTestStreamState 构造测试用流快照
func makeTestStreamState() *stream.State {
	return &stream.State{
		Index:           7,
		Direction:       stream.DirectionOutbound,
		LocalDomain:     "my-pc",
		LocalAddr:       stream.NewAddr("tcp", "1:13389"),
		RemoteDomain:    "remote-pc",
		RemoteAddr:      stream.NewAddr("tcp", "2:50001"),
		BytesRead:       1024,
		BytesWritten:    512,
		Created:         time.Now().Add(-5 * time.Second),
		SentBufferCount: 3,
		RecvBufferCount: 4,
		RecvDataSize:    2048,
		RecvAckTotal:    1024,
		SentAckTotal:    512,
	}
}

func TestHandleStreamsEnriched(t *testing.T) {
	closed := makeTestStreamState()
	closed.Index = 8
	closed.IsClosed = true
	closed.ReadClosed = true
	closed.WriteClosed = true
	closed.CloseReason = "remote"
	closed.Closed = time.Now()

	b := &streamBackend{
		streams: []*DataStreamState{{
			Network: "mynet",
			Actives: []*stream.State{makeTestStreamState()},
			Closeds: []*stream.State{closed},
		}},
		serviceName: "rdp-forward",
	}
	s := startTestServerWith(t, b, Config{Meta: testMeta})

	resp := doRequest(t, "GET", "http://"+s.Addr()+"/api/v1/streams", "", nil)
	env := decodeEnvelope(t, resp)
	if env.ErrCode != 0 {
		t.Fatalf("want ErrCode=0, got %d (%s)", env.ErrCode, env.ErrMsg)
	}
	var dtos []StreamStateDTO
	if err := json.Unmarshal(env.Data, &dtos); err != nil {
		t.Fatalf("unmarshal streams: %v", err)
	}
	if len(dtos) != 2 {
		t.Fatalf("want 2 streams, got %d", len(dtos))
	}

	// 活跃流：完整字段透传 + 归属服务标注；首次采样速率为 0
	active := dtos[0]
	if active.Index != 7 || active.Direction != "outbound" {
		t.Errorf("active index/direction mismatch: %+v", active)
	}
	if active.ReadBytes != 1024 || active.WriteBytes != 512 {
		t.Errorf("active bytes mismatch: %+v", active)
	}
	if active.SentBufferCount != 3 || active.RecvBufferCount != 4 ||
		active.RecvDataSize != 2048 || active.RecvAckTotal != 1024 || active.SentAckTotal != 512 {
		t.Errorf("active counters mismatch: %+v", active)
	}
	if active.ReadBps != 0 || active.WriteBps != 0 {
		t.Errorf("first sample rate should be 0, got read=%v write=%v", active.ReadBps, active.WriteBps)
	}
	if active.ServiceName != "rdp-forward" {
		t.Errorf("active serviceName mismatch: %q", active.ServiceName)
	}

	// 已关闭流：半关闭与关闭原因透传
	closedDTO := dtos[1]
	if !closedDTO.IsClosed || !closedDTO.ReadClosed || !closedDTO.WriteClosed {
		t.Errorf("closed flags mismatch: %+v", closedDTO)
	}
	if closedDTO.CloseReason != "remote" {
		t.Errorf("closeReason mismatch: %q", closedDTO.CloseReason)
	}
	if closedDTO.ServiceName != "rdp-forward" {
		t.Errorf("closed serviceName mismatch: %q", closedDTO.ServiceName)
	}
}

func TestToStreamDTOMapping(t *testing.T) {
	st := makeTestStreamState()
	dto := toStreamDTO("mynet", st)

	if dto.Network != "mynet" || dto.Index != 7 {
		t.Errorf("network/index mismatch: %+v", dto)
	}
	if dto.Direction != "outbound" {
		t.Errorf("direction: want outbound, got %q", dto.Direction)
	}
	if dto.LocalDomain != "my-pc" || dto.LocalAddr != "1:13389" ||
		dto.RemoteDomain != "remote-pc" || dto.RemoteAddr != "2:50001" {
		t.Errorf("addr fields mismatch: %+v", dto)
	}
	if dto.ReadBytes != 1024 || dto.WriteBytes != 512 || dto.IsClosed {
		t.Errorf("bytes/isClosed mismatch: %+v", dto)
	}
	if dto.AliveMs <= 0 {
		t.Errorf("aliveMs should be > 0, got %d", dto.AliveMs)
	}
	// 速率与归属服务保持零值，由调用方二次填充
	if dto.ReadBps != 0 || dto.WriteBps != 0 || dto.ServiceName != "" {
		t.Errorf("rate/serviceName should be zero-value: %+v", dto)
	}

	// 入站方向映射
	st.Direction = stream.DirectionInbound
	if got := toStreamDTO("mynet", st).Direction; got != "inbound" {
		t.Errorf("direction: want inbound, got %q", got)
	}
	// 未知方向映射
	st.Direction = 0
	if got := toStreamDTO("mynet", st).Direction; got != "unknown" {
		t.Errorf("direction: want unknown, got %q", got)
	}
}

func TestToNetworkDTOExtended(t *testing.T) {
	dto := toNetworkDTO(vnet.NetworkReport{
		Name:         "mynet",
		BytesRead:    12345,
		BytesWritten: 67890,
		RTT:          52 * time.Millisecond,
	})
	if dto.BytesRead != 12345 || dto.BytesWritten != 67890 {
		t.Errorf("bytes mismatch: %+v", dto)
	}
	if dto.RTTMs != 52 {
		t.Errorf("rttMs: want 52, got %d", dto.RTTMs)
	}
}

// readWSEvent 从测试客户端的发送通道读一条事件
func readWSEvent(t *testing.T, c *WSClient) Event {
	t.Helper()
	select {
	case msg := <-c.send:
		var evt Event
		if err := json.Unmarshal(msg, &evt); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		return evt
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting ws event")
		return Event{}
	}
}

// collectWSEvents 读取 n 条事件并按类型索引（每 tick 会伴随 service.stats/network.stats 等周期事件）
func collectWSEvents(t *testing.T, c *WSClient, n int) map[string]Event {
	t.Helper()
	byType := make(map[string]Event, n)
	for i := 0; i < n; i++ {
		evt := readWSEvent(t, c)
		byType[evt.Type] = evt
	}
	return byType
}

func TestPollerStreamStatsAndUpdate(t *testing.T) {
	st := makeTestStreamState()
	b := &streamBackend{
		streams:     []*DataStreamState{{Network: "mynet", Actives: []*stream.State{st}}},
		serviceName: "rdp-forward",
		pipeID:      "rdp-forward#3",
	}
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	poller := NewStatePoller(b, hub, nil, time.Minute, time.Minute)

	c := &WSClient{send: make(chan []byte, 64)}
	hub.addClient(c)

	// 首轮：service.stats / network.stats 慢节拍快照 + stream.stats 快节拍快照 + stream.update 差分（opened）
	poller.poll()
	poller.pollStreamStats()

	evts := collectWSEvents(t, c, 4)
	stats, ok := evts["stream.stats"]
	if !ok {
		t.Fatalf("want stream.stats among %v", evts)
	}
	if stats.Topic != "stream" || stats.Seq <= 0 {
		t.Errorf("stream.stats should carry topic=stream and seq>0, got %+v", stats)
	}
	statsData := stats.Data.(map[string]interface{})
	if statsData["network"] != "mynet" {
		t.Errorf("stats network mismatch: %v", statsData["network"])
	}
	streams := statsData["streams"].([]interface{})
	if len(streams) != 1 {
		t.Fatalf("want 1 stream in stats, got %d", len(streams))
	}
	dto0 := streams[0].(map[string]interface{})
	if dto0["index"].(float64) != 7 || dto0["direction"] != "outbound" {
		t.Errorf("stats dto index/direction mismatch: %v", dto0)
	}
	if dto0["serviceName"] != "rdp-forward" {
		t.Errorf("stats dto serviceName mismatch: %v", dto0["serviceName"])
	}
	if dto0["pipeId"] != "rdp-forward#3" {
		t.Errorf("stats dto pipeId mismatch: %v", dto0["pipeId"])
	}
	if dto0["readBps"].(float64) != 0 {
		t.Errorf("first poll readBps should be 0, got %v", dto0["readBps"])
	}
	if dto0["createdAtMs"].(float64) <= 0 {
		t.Errorf("createdAtMs should be > 0, got %v", dto0["createdAtMs"])
	}

	update, ok := evts["stream.update"]
	if !ok {
		t.Fatalf("want stream.update among %v", evts)
	}
	opened := update.Data.(map[string]interface{})["opened"].([]interface{})
	if len(opened) != 1 {
		t.Fatalf("want 1 opened stream, got %d", len(opened))
	}
	openedDTO := opened[0].(map[string]interface{})
	if openedDTO["serviceName"] != "rdp-forward" {
		t.Errorf("opened dto serviceName mismatch: %v", openedDTO["serviceName"])
	}
	if openedDTO["pipeId"] != "rdp-forward#3" {
		t.Errorf("opened dto pipeId mismatch: %v", openedDTO["pipeId"])
	}

	// 次轮：字节数增长 → 速率非零；流集合不变 → 不再推 stream.update
	st.BytesRead += 2000
	st.BytesWritten += 1000
	poller.pollStreamStats()

	evts = collectWSEvents(t, c, 1) // 仅 stream.stats
	stats = evts["stream.stats"]
	streams = stats.Data.(map[string]interface{})["streams"].([]interface{})
	dto0 = streams[0].(map[string]interface{})
	if dto0["readBytes"].(float64) != 3024 || dto0["writeBytes"].(float64) != 1512 {
		t.Errorf("second poll bytes should be updated, got read=%v write=%v",
			dto0["readBytes"], dto0["writeBytes"])
	}
	if dto0["readBps"].(float64) <= 0 || dto0["writeBps"].(float64) <= 0 {
		t.Errorf("second poll rates should be > 0, got read=%v write=%v",
			dto0["readBps"], dto0["writeBps"])
	}

	select {
	case msg := <-c.send:
		t.Fatalf("unexpected extra event: %s", msg)
	default:
	}
}

// TestPollerFastStatsTicker 验证 stream.stats 由独立快节拍驱动：
// 慢节拍设为足够长（测试期间不触发），快节拍 50ms，字节数变化应在秒级以内送达
func TestPollerFastStatsTicker(t *testing.T) {
	b := &atomicStreamBackend{}
	b.readBytes.Store(1024)
	hub := NewWSHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	poller := NewStatePoller(b, hub, nil, time.Hour, 50*time.Millisecond)
	poller.Start()
	defer poller.Stop()

	c := &WSClient{send: make(chan []byte, 64)}
	hub.addClient(c)

	// strict=true 时遇到非 stream.stats 事件（慢节拍产物）直接失败，
	// 用于验证慢节拍不会混入快节拍通道
	nextStats := func(strict bool) Event {
		deadline := time.After(2 * time.Second)
		for {
			select {
			case msg := <-c.send:
				var evt Event
				if err := json.Unmarshal(msg, &evt); err != nil {
					t.Fatalf("unmarshal event: %v", err)
				}
				if evt.Type == "stream.stats" {
					return evt
				}
				if strict {
					t.Fatalf("unexpected slow-tick event on fast ticker: %v", evt.Type)
				}
			case <-deadline:
				t.Fatal("timeout waiting stream.stats on fast ticker")
			}
		}
	}

	nextStats(true) // 首个 stream.stats 由快节拍独立发出

	// 字节数增长后，后续快节拍推送应反映新值（readed/wrote 实时推送链路验证）。
	// 变更前已采样的 tick 可能仍携带旧值，循环等到新值为止
	b.readBytes.Add(4096)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg := <-c.send:
			var evt Event
			if err := json.Unmarshal(msg, &evt); err != nil {
				t.Fatalf("unmarshal event: %v", err)
			}
			if evt.Type != "stream.stats" {
				continue
			}
			streams := evt.Data.(map[string]interface{})["streams"].([]interface{})
			if got := streams[0].(map[string]interface{})["readBytes"].(float64); got == 1024+4096 {
				return // 收到更新后的字节数，链路验证通过
			}
		case <-deadline:
			t.Fatal("timeout waiting stream.stats with updated readBytes")
		}
	}
}

// atomicStreamBackend 字节数可原子变更的流后端（快节拍测试用，避免数据竞争）
type atomicStreamBackend struct {
	fakeBackend
	readBytes atomic.Int64
}

func (b *atomicStreamBackend) GetDataStreamState(limit int, networks ...string) []*DataStreamState {
	st := makeTestStreamState()
	st.BytesRead = b.readBytes.Load()
	return []*DataStreamState{{Network: "mynet", Actives: []*stream.State{st}}}
}
