package vnet

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/net-agent/flex/v3/node"
	"github.com/net-agent/flex/v3/packet"
	"github.com/net-agent/flex/v3/stream"
)

// mockHalfCloseConn 在 net.Conn 上附加 CloseRead/CloseWrite 标记
// （net.Pipe 连接本身不支持半关闭，用它模拟 *net.TCPConn 的能力形状）
type mockHalfCloseConn struct {
	net.Conn
	readClosed  atomic.Bool
	writeClosed atomic.Bool
}

func (c *mockHalfCloseConn) CloseRead() error {
	c.readClosed.Store(true)
	return nil
}

func (c *mockHalfCloseConn) CloseWrite() error {
	c.writeClosed.Store(true)
	return nil
}

// newPipeTracked 用 net.Pipe 造一对连接，把一端注册进 tracker，返回 (tracked, 对端)
func newPipeTracked(tr *connTracker, direction stream.Direction) (net.Conn, net.Conn) {
	c1, c2 := net.Pipe()
	return tr.track(c1, direction), c2
}

func TestTrackedConn_ReadWriteByteCounts(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	tc, peer := newPipeTracked(tr, stream.DirectionOutbound)
	defer tc.Close()
	defer peer.Close()

	// 对端写 10 字节 → tracked 读
	go func() {
		peer.Write([]byte("0123456789"))
	}()
	buf := make([]byte, 64)
	n, err := tc.Read(buf)
	if err != nil || n != 10 {
		t.Fatalf("Read = %d, %v, want 10, nil", n, err)
	}

	// tracked 写 5 字节 → 对端读
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		tc.Write([]byte("hello"))
	}()
	n, err = peer.Read(buf)
	if err != nil || n != 5 {
		t.Fatalf("peer Read = %d, %v, want 5, nil", n, err)
	}
	// 等写 goroutine 返回（字节计数在 Write 返回前累加），避免断言竞争
	<-writeDone

	actives, _ := tr.GetStreamStates()
	if len(actives) != 1 {
		t.Fatalf("actives = %d, want 1", len(actives))
	}
	st := actives[0]
	if st.BytesRead != 10 {
		t.Errorf("BytesRead = %d, want 10", st.BytesRead)
	}
	if st.BytesWritten != 5 {
		t.Errorf("BytesWritten = %d, want 5", st.BytesWritten)
	}
}

func TestTrackedConn_CloseMigratesToClosedReasonLocal(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	tc, peer := newPipeTracked(tr, stream.DirectionOutbound)
	defer peer.Close()

	if err := tc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	actives, closeds := tr.GetStreamStates()
	if len(actives) != 0 {
		t.Errorf("actives = %d, want 0", len(actives))
	}
	if len(closeds) != 1 {
		t.Fatalf("closeds = %d, want 1", len(closeds))
	}
	st := closeds[0]
	if st.CloseReason != "local" {
		t.Errorf("CloseReason = %q, want %q", st.CloseReason, "local")
	}
	if !st.IsClosed {
		t.Error("IsClosed should be true")
	}
	if !st.ReadClosed || !st.WriteClosed {
		t.Error("ReadClosed/WriteClosed should both be true after Close")
	}
	if st.Closed.IsZero() {
		t.Error("Closed time should be set")
	}

	// Close 幂等：重复关闭不产生第二条历史
	tc.Close()
	_, closeds = tr.GetStreamStates()
	if len(closeds) != 1 {
		t.Errorf("closeds = %d after second Close, want 1", len(closeds))
	}
}

func TestTrackedConn_EOFRecordsReasonRemote(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	tc, peer := newPipeTracked(tr, stream.DirectionOutbound)

	// 对端关闭 → 本地 Read 收到 io.EOF
	peer.Close()
	buf := make([]byte, 16)
	if _, err := tc.Read(buf); !errors.Is(err, io.EOF) {
		t.Fatalf("Read err = %v, want io.EOF", err)
	}

	// EOF 只归因，不主动关连接：连接仍在 actives
	actives, _ := tr.GetStreamStates()
	if len(actives) != 1 {
		t.Fatalf("actives = %d after EOF, want 1", len(actives))
	}
	if actives[0].CloseReason != "remote" {
		t.Errorf("CloseReason = %q, want %q", actives[0].CloseReason, "remote")
	}

	// 之后本地 Close，reason 保持 first-wins 的 "remote"
	tc.Close()
	_, closeds := tr.GetStreamStates()
	if len(closeds) != 1 {
		t.Fatalf("closeds = %d, want 1", len(closeds))
	}
	if closeds[0].CloseReason != "remote" {
		t.Errorf("CloseReason = %q after local Close, want %q (first-wins)", closeds[0].CloseReason, "remote")
	}
}

func TestTrackedConn_WriteErrorRecordsReasonRemote(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	tc, peer := newPipeTracked(tr, stream.DirectionOutbound)
	defer tc.Close()

	peer.Close()
	if _, err := tc.Write([]byte("x")); err == nil {
		t.Fatal("Write should fail after peer closed")
	}

	actives, _ := tr.GetStreamStates()
	if len(actives) != 1 {
		t.Fatalf("actives = %d, want 1", len(actives))
	}
	if actives[0].CloseReason != "remote" {
		t.Errorf("CloseReason = %q, want %q", actives[0].CloseReason, "remote")
	}
}

func TestConnTracker_HalfCloseTypeSelection(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)

	// 底层支持半关闭（mockHalfCloseConn）→ 包装后可断言 CloseRead/CloseWrite
	c1, p1 := net.Pipe()
	defer p1.Close()
	wrapped := tr.track(&mockHalfCloseConn{Conn: c1}, stream.DirectionOutbound)
	if _, ok := wrapped.(interface{ CloseRead() error }); !ok {
		t.Error("half-close capable conn should keep CloseRead assertion")
	}
	if _, ok := wrapped.(interface{ CloseWrite() error }); !ok {
		t.Error("half-close capable conn should keep CloseWrite assertion")
	}
	wrapped.Close()

	// 底层不支持（裸 net.Pipe）→ 包装后断言不到，避免 RelayConns 误判
	c2, p2 := net.Pipe()
	defer p2.Close()
	wrapped2 := tr.track(c2, stream.DirectionOutbound)
	if _, ok := wrapped2.(interface{ CloseRead() error }); ok {
		t.Error("pipe conn wrapper must NOT expose CloseRead")
	}
	if _, ok := wrapped2.(interface{ CloseWrite() error }); ok {
		t.Error("pipe conn wrapper must NOT expose CloseWrite")
	}
	wrapped2.Close()
}

func TestTrackedHalfConn_BothHalfCloseFinalizes(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	raw, peer := net.Pipe()
	defer peer.Close()
	hc := &mockHalfCloseConn{Conn: raw}
	wrapped := tr.track(hc, stream.DirectionOutbound).(interface {
		net.Conn
		CloseRead() error
		CloseWrite() error
	})

	// 只关读：仍在 actives，readClosed=true
	if err := wrapped.CloseRead(); err != nil {
		t.Fatalf("CloseRead failed: %v", err)
	}
	if !hc.readClosed.Load() {
		t.Error("underlying CloseRead not forwarded")
	}
	actives, closeds := tr.GetStreamStates()
	if len(actives) != 1 || len(closeds) != 0 {
		t.Fatalf("after CloseRead: actives=%d closeds=%d, want 1/0", len(actives), len(closeds))
	}
	if !actives[0].ReadClosed || actives[0].WriteClosed {
		t.Errorf("half-close flags = read:%v write:%v, want true/false",
			actives[0].ReadClosed, actives[0].WriteClosed)
	}

	// 再关写：双向都关 → finalize 迁移到 closeds，reason 缺省 "local"
	if err := wrapped.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite failed: %v", err)
	}
	if !hc.writeClosed.Load() {
		t.Error("underlying CloseWrite not forwarded")
	}
	actives, closeds = tr.GetStreamStates()
	if len(actives) != 0 || len(closeds) != 1 {
		t.Fatalf("after both half-close: actives=%d closeds=%d, want 0/1", len(actives), len(closeds))
	}
	st := closeds[0]
	if st.CloseReason != "local" {
		t.Errorf("CloseReason = %q, want %q", st.CloseReason, "local")
	}
	if !st.IsClosed || !st.ReadClosed || !st.WriteClosed {
		t.Error("finalized state should be fully closed")
	}
}

func TestConnTracker_ClosedHistoryBounded(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	total := maxClosedStreamHist + 10

	peers := make([]net.Conn, 0, total)
	for i := 0; i < total; i++ {
		tc, peer := newPipeTracked(tr, stream.DirectionOutbound)
		peers = append(peers, peer)
		tc.Close()
	}
	for _, p := range peers {
		p.Close()
	}

	_, closeds := tr.GetStreamStates()
	if len(closeds) != maxClosedStreamHist {
		t.Fatalf("closeds = %d, want %d", len(closeds), maxClosedStreamHist)
	}
	// 最旧的 10 条（id 1..10）被淘汰，现存最旧为 id 11
	if closeds[0].Index != 11 {
		t.Errorf("closeds[0].Index = %d, want 11 (oldest evicted)", closeds[0].Index)
	}
}

func TestConnTracker_GetStreamStatesFields(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)

	// 用固定地址的 mock conn 验证合成字段
	mc := &mockAddrConn{
		local:  mockAddr("10.0.0.1:1000"),
		remote: mockAddr("10.0.0.2:2000"),
	}
	wrapped := tr.track(mc, stream.DirectionOutbound)
	defer wrapped.Close()

	actives, _ := tr.GetStreamStates()
	if len(actives) != 1 {
		t.Fatalf("actives = %d, want 1", len(actives))
	}
	st := actives[0]
	if st.Index != 1 {
		t.Errorf("Index = %d, want 1 (自增从 1 开始)", st.Index)
	}
	if st.Direction != stream.DirectionOutbound {
		t.Errorf("Direction = %d, want %d", st.Direction, stream.DirectionOutbound)
	}
	if st.LocalAddr.String() != "10.0.0.1:1000" {
		t.Errorf("LocalAddr = %q, want %q", st.LocalAddr.String(), "10.0.0.1:1000")
	}
	if st.RemoteAddr.String() != "10.0.0.2:2000" {
		t.Errorf("RemoteAddr = %q, want %q", st.RemoteAddr.String(), "10.0.0.2:2000")
	}
	if st.LocalAddr.Network() != "tcp-test" {
		t.Errorf("LocalAddr.Network() = %q, want tcpNetwork name %q", st.LocalAddr.Network(), "tcp-test")
	}
	if st.LocalDomain != "" || st.RemoteDomain != "" {
		t.Error("LocalDomain/RemoteDomain should be empty for tcp conns")
	}
	if st.Created.IsZero() {
		t.Error("Created should be set")
	}
	if st.IsClosed || st.CloseReason != "" {
		t.Error("active conn should not be marked closed")
	}
}

// mockAddrConn 固定地址的桩 conn（Read/Write 直接报 EOF，仅用于注册合成）
type mockAddrConn struct {
	local  net.Addr
	remote net.Addr
}

func (c *mockAddrConn) Read(p []byte) (int, error)         { return 0, io.EOF }
func (c *mockAddrConn) Write(p []byte) (int, error)        { return 0, io.EOF }
func (c *mockAddrConn) Close() error                       { return nil }
func (c *mockAddrConn) LocalAddr() net.Addr                { return c.local }
func (c *mockAddrConn) RemoteAddr() net.Addr               { return c.remote }
func (c *mockAddrConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockAddrConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockAddrConn) SetWriteDeadline(t time.Time) error { return nil }

func TestConnTracker_ConnID(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	tc1, p1 := newPipeTracked(tr, stream.DirectionOutbound)
	defer p1.Close()
	tc2, p2 := newPipeTracked(tr, stream.DirectionOutbound)
	defer p2.Close()
	defer tc1.Close()
	defer tc2.Close()

	id1 := tc1.(interface{ ConnID() int64 }).ConnID()
	id2 := tc2.(interface{ ConnID() int64 }).ConnID()
	if id1 != 1 || id2 != 2 {
		t.Errorf("ConnID = %d, %d, want 1, 2", id1, id2)
	}
}

func TestConnTracker_CloseStream(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	tc1, p1 := newPipeTracked(tr, stream.DirectionOutbound)
	defer p1.Close()
	tc2, p2 := newPipeTracked(tr, stream.DirectionOutbound)
	defer p2.Close()
	defer tc2.Close()

	// 找不到
	if tr.CloseStream(9999) {
		t.Error("CloseStream(9999) should return false")
	}

	// 找到：reason 记 "manual"，底层连接被关闭
	if !tr.CloseStream(1) {
		t.Fatal("CloseStream(1) should return true")
	}
	actives, closeds := tr.GetStreamStates()
	if len(actives) != 1 || len(closeds) != 1 {
		t.Fatalf("actives=%d closeds=%d, want 1/1", len(actives), len(closeds))
	}
	if closeds[0].CloseReason != "manual" {
		t.Errorf("CloseReason = %q, want %q", closeds[0].CloseReason, "manual")
	}
	if _, err := tc1.Write([]byte("x")); err == nil {
		t.Error("underlying conn should be closed by CloseStream")
	}

	// 已迁移出 actives，再关返回 false
	if tr.CloseStream(1) {
		t.Error("CloseStream(1) after close should return false")
	}
}

func TestConnTracker_CloseAllReasonStopped(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	tc1, p1 := newPipeTracked(tr, stream.DirectionOutbound)
	defer p1.Close()
	// tc2 无需直接引用，由 CloseAll 负责关闭
	_, p2 := newPipeTracked(tr, stream.DirectionOutbound)
	defer p2.Close()

	// tc1 先收到 EOF（reason 预占 "remote"），验证 CloseAll 的 first-wins
	p1.Close()
	tc1.Read(make([]byte, 1))

	tr.CloseAll("stopped")

	actives, closeds := tr.GetStreamStates()
	if len(actives) != 0 {
		t.Errorf("actives = %d after CloseAll, want 0", len(actives))
	}
	if len(closeds) != 2 {
		t.Fatalf("closeds = %d, want 2", len(closeds))
	}
	reasons := map[int32]string{closeds[0].Index: closeds[0].CloseReason, closeds[1].Index: closeds[1].CloseReason}
	if reasons[1] != "remote" {
		t.Errorf("conn1 CloseReason = %q, want %q (first-wins over stopped)", reasons[1], "remote")
	}
	if reasons[2] != "stopped" {
		t.Errorf("conn2 CloseReason = %q, want %q", reasons[2], "stopped")
	}
}

func TestTrackedListener_AcceptRegistersInbound(t *testing.T) {
	tr := newConnTracker("tcp-test", nil)
	ml := NewMockListener("mock:0")
	tl := &trackedListener{Listener: ml, tracker: tr}

	c1, c2 := net.Pipe()
	defer c2.Close()
	go func() { ml.acceptCh <- c1 }()

	conn, err := tl.Accept()
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	defer conn.Close()

	actives, _ := tr.GetStreamStates()
	if len(actives) != 1 {
		t.Fatalf("actives = %d, want 1", len(actives))
	}
	if actives[0].Direction != stream.DirectionInbound {
		t.Errorf("Direction = %d, want inbound %d", actives[0].Direction, stream.DirectionInbound)
	}
}

func TestTcpNetwork_LoopbackEndToEnd(t *testing.T) {
	tn := NewTcpNetwork("tcp-e2e", nil)

	ln, err := tn.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	acceptCh := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			acceptCh <- conn
		}
	}()

	dialConn, err := tn.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	// 真实 TCP 连接包装后必须保活半关闭断言（RelayConns 依赖）
	if _, ok := dialConn.(interface{ CloseWrite() error }); !ok {
		t.Error("dialed tcp conn should keep CloseWrite assertion")
	}

	var acceptConn net.Conn
	select {
	case acceptConn = <-acceptCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Accept timeout")
	}

	// 双向各传一段数据
	if _, err := dialConn.Write([]byte("ping")); err != nil {
		t.Fatalf("dial Write failed: %v", err)
	}
	buf := make([]byte, 16)
	dialConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := acceptConn.Read(buf); err != nil {
		t.Fatalf("accept Read failed: %v", err)
	}
	if _, err := acceptConn.Write([]byte("pong!!")); err != nil {
		t.Fatalf("accept Write failed: %v", err)
	}
	if _, err := dialConn.Read(buf); err != nil {
		t.Fatalf("dial Read failed: %v", err)
	}

	actives, _ := tn.GetStreamStates()
	if len(actives) != 2 {
		t.Fatalf("actives = %d, want 2 (outbound + inbound)", len(actives))
	}
	byDirection := map[stream.Direction]*stream.State{}
	for _, st := range actives {
		byDirection[st.Direction] = st
	}
	out := byDirection[stream.DirectionOutbound]
	in := byDirection[stream.DirectionInbound]
	if out == nil || in == nil {
		t.Fatal("should have one outbound and one inbound conn")
	}
	if out.BytesRead != 6 || out.BytesWritten != 4 {
		t.Errorf("outbound bytes = read:%d written:%d, want 6/4", out.BytesRead, out.BytesWritten)
	}
	if in.BytesRead != 4 || in.BytesWritten != 6 {
		t.Errorf("inbound bytes = read:%d written:%d, want 4/6", in.BytesRead, in.BytesWritten)
	}
	if out.LocalAddr.String() == "" || out.RemoteAddr.String() == "" {
		t.Error("addr text should be captured")
	}

	// 手动关闭 outbound 连接
	if !tn.CloseStream(out.Index) {
		t.Fatal("CloseStream should find outbound conn")
	}
	actives, closeds := tn.GetStreamStates()
	if len(actives) != 1 || len(closeds) != 1 {
		t.Fatalf("after CloseStream: actives=%d closeds=%d, want 1/1", len(actives), len(closeds))
	}
	if closeds[0].CloseReason != "manual" {
		t.Errorf("CloseReason = %q, want %q", closeds[0].CloseReason, "manual")
	}

	// Stop 关闭剩余连接（inbound 已观察到对端关闭，reason 归 "remote" 或 "stopped" 均可接受，
	// 但必须在 CloseAll 后进入历史）
	tn.Stop()
	actives, closeds = tn.GetStreamStates()
	if len(actives) != 0 {
		t.Errorf("actives = %d after Stop, want 0", len(actives))
	}
	if len(closeds) != 2 {
		t.Fatalf("closeds = %d after Stop, want 2", len(closeds))
	}

	// Meta 保持本地网络语义：流量/RTT 零值
	meta := tn.Meta()
	if meta.BytesRead != 0 || meta.BytesWritten != 0 || meta.RTT != 0 {
		t.Error("tcp Meta should keep zero BytesRead/BytesWritten/RTT")
	}
}

func TestFlexNetwork_CloseStreamAndMetaWithoutNode(t *testing.T) {
	// 未 Serve 的 Session：GetNode 为 nil（等价于重连中）
	sess := node.NewSession(func() (packet.Conn, error) {
		return nil, errors.New("no relay")
	}, node.SessionConfig{Domain: "test-node"})
	fnet := &flexNetwork{name: "flex-test", session: sess}

	if fnet.CloseStream(1) {
		t.Error("CloseStream should return false when node is nil")
	}

	meta := fnet.Meta()
	if meta.BytesRead != 0 || meta.BytesWritten != 0 || meta.RTT != 0 {
		t.Error("Meta should keep zero BytesRead/BytesWritten/RTT when node is nil")
	}
	if meta.State != "ready" {
		t.Errorf("State = %q, want %q", meta.State, "ready")
	}
}

func TestConnTracker_Concurrent(t *testing.T) {
	tr := newConnTracker("tcp-race", nil)

	const conns = 16
	peers := make([]net.Conn, 0, conns)
	tracked := make([]net.Conn, 0, conns)
	for i := 0; i < conns; i++ {
		tc, peer := newPipeTracked(tr, stream.DirectionOutbound)
		tracked = append(tracked, tc)
		peers = append(peers, peer)
	}
	defer func() {
		for _, p := range peers {
			p.Close()
		}
	}()

	var wg sync.WaitGroup
	// 并发读写
	for i := 0; i < conns; i++ {
		wg.Add(2)
		go func(tc net.Conn) {
			defer wg.Done()
			buf := make([]byte, 8)
			for j := 0; j < 20; j++ {
				tc.Read(buf)
			}
		}(tracked[i])
		go func(peer net.Conn) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				peer.Write([]byte("12345678"))
			}
		}(peers[i])
	}
	// 并发查询与关闭
	wg.Add(3)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			tr.GetStreamStates()
		}
	}()
	go func() {
		defer wg.Done()
		for id := int32(1); id <= conns; id++ {
			tr.CloseStream(id)
		}
	}()
	go func() {
		defer wg.Done()
		for _, tc := range tracked {
			tc.Close()
		}
	}()
	wg.Wait()

	// 所有连接最终都在历史里
	actives, closeds := tr.GetStreamStates()
	if len(actives) != 0 {
		t.Errorf("actives = %d, want 0", len(actives))
	}
	if len(closeds) != conns {
		t.Errorf("closeds = %d, want %d", len(closeds), conns)
	}
}

// traceRecorder 记录 StreamTrace 回调触发的快照序列
type traceRecorder struct {
	opened []*stream.State
	closed []*stream.State
}

func (r *traceRecorder) trace() *StreamTrace {
	return &StreamTrace{
		OnStreamOpen:   func(st *stream.State) { r.opened = append(r.opened, st) },
		OnStreamClosed: func(st *stream.State) { r.closed = append(r.closed, st) },
	}
}

func TestStreamTrace_OpenAndCloseCallbacks(t *testing.T) {
	rec := &traceRecorder{}
	tr := newConnTracker("tcp-trace", rec.trace())

	tc, peer := newPipeTracked(tr, stream.DirectionInbound)
	defer peer.Close()

	// 注册即触发 OnStreamOpen：恰好一次，快照为初始状态
	if len(rec.opened) != 1 {
		t.Fatalf("OnStreamOpen fired %d times, want 1", len(rec.opened))
	}
	openSt := rec.opened[0]
	if openSt.Index != 1 || openSt.Direction != stream.DirectionInbound {
		t.Errorf("open snapshot index/direction mismatch: %+v", openSt)
	}
	if openSt.IsClosed || openSt.BytesRead != 0 || openSt.BytesWritten != 0 {
		t.Errorf("open snapshot should be initial state: %+v", openSt)
	}
	if len(rec.closed) != 0 {
		t.Fatalf("OnStreamClosed should not fire before close, got %d", len(rec.closed))
	}

	// 写入数据后关闭：OnStreamClosed 恰好一次，快照含最终字节数与关闭原因
	go func() { peer.Read(make([]byte, 8)) }()
	if _, err := tc.Write([]byte("hello")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tc.Close()
	tc.Close() // 幂等：重复关闭不重复触发

	if len(rec.closed) != 1 {
		t.Fatalf("OnStreamClosed fired %d times, want 1", len(rec.closed))
	}
	closeSt := rec.closed[0]
	if closeSt.Index != openSt.Index {
		t.Errorf("close snapshot index = %d, want same as open %d", closeSt.Index, openSt.Index)
	}
	if !closeSt.IsClosed || closeSt.CloseReason != "local" {
		t.Errorf("close snapshot flags mismatch: %+v", closeSt)
	}
	if closeSt.BytesWritten != 5 {
		t.Errorf("close snapshot BytesWritten = %d, want 5", closeSt.BytesWritten)
	}
}

func TestStreamTrace_HalfCloseFiresClosedOnce(t *testing.T) {
	rec := &traceRecorder{}
	tr := newConnTracker("tcp-trace", rec.trace())

	raw, peer := net.Pipe()
	defer peer.Close()
	wrapped := tr.track(&mockHalfCloseConn{Conn: raw}, stream.DirectionOutbound).(interface {
		net.Conn
		CloseRead() error
		CloseWrite() error
	})

	// 单向半关闭不触发 closed
	if err := wrapped.CloseRead(); err != nil {
		t.Fatalf("CloseRead failed: %v", err)
	}
	if len(rec.closed) != 0 {
		t.Fatalf("OnStreamClosed should not fire on single half-close, got %d", len(rec.closed))
	}

	// 双向齐备 → finalize → 恰好一次 closed
	if err := wrapped.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite failed: %v", err)
	}
	if len(rec.opened) != 1 || len(rec.closed) != 1 {
		t.Fatalf("opened=%d closed=%d, want 1/1", len(rec.opened), len(rec.closed))
	}

	// 之后的 Close 不再重复触发
	wrapped.Close()
	if len(rec.closed) != 1 {
		t.Errorf("OnStreamClosed after final Close = %d, want 1", len(rec.closed))
	}
}

func TestStreamTrace_NilSafe(t *testing.T) {
	// trace 为 nil / 回调为 nil 时均不得 panic
	tr := newConnTracker("tcp-trace", nil)
	tc, peer := newPipeTracked(tr, stream.DirectionOutbound)
	peer.Close()
	tc.Close()

	tr2 := newConnTracker("tcp-trace", &StreamTrace{})
	tc2, peer2 := newPipeTracked(tr2, stream.DirectionOutbound)
	peer2.Close()
	tc2.Close()
}
