package vservice

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/net-agent/cipherconn"
	"github.com/net-agent/flex/v3/stream"
)

// mockConnIDConn 模拟 tcp 网络的追踪连接（ConnID 结构化接口）
type mockConnIDConn struct {
	net.Conn
	id     int64
	closed int32
}

func (c *mockConnIDConn) ConnID() int64 { return c.id }
func (c *mockConnIDConn) Close() error {
	atomic.AddInt32(&c.closed, 1)
	return c.Conn.Close()
}

// mockStreamConn 模拟 flex 流连接（GetState 结构化接口）
type mockStreamConn struct {
	net.Conn
	state *stream.State
}

func (c *mockStreamConn) GetState() *stream.State { return c.state }

// mockHalfCloseConn 支持双向半关闭的追踪连接
type mockHalfCloseConn struct {
	net.Conn
	id               int64
	closed           int32
	closeReadCalled  int32
	closeWriteCalled int32
}

func (c *mockHalfCloseConn) ConnID() int64 { return c.id }
func (c *mockHalfCloseConn) Close() error {
	atomic.AddInt32(&c.closed, 1)
	return c.Conn.Close()
}
func (c *mockHalfCloseConn) CloseRead() error {
	atomic.AddInt32(&c.closeReadCalled, 1)
	return nil
}
func (c *mockHalfCloseConn) CloseWrite() error {
	atomic.AddInt32(&c.closeWriteCalled, 1)
	return nil
}

// pipePair 返回一对内存管道连接，随测试结束自动关闭
func pipePair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() {
		a.Close()
		b.Close()
	})
	return a, b
}

func TestNetworkOfURL(t *testing.T) {
	tests := []struct {
		rawurl string
		want   string
	}{
		{"alias://vhost:80?authcode=x", "alias"},
		{"tcp://127.0.0.1:80", "tcp"},
		{"vtcp://host.link1:3389?authcode=k%2F", "vtcp"},
		{"socks5://0.0.0.0:1080", "socks5"},
		{"", ""},
		{"%zz", ""},            // 非法转义，解析失败
		{"127.0.0.1:8080", ""}, // 无 scheme，解析失败
	}
	for _, tt := range tests {
		if got := networkOfURL(tt.rawurl); got != tt.want {
			t.Errorf("networkOfURL(%q) = %q, want %q", tt.rawurl, got, tt.want)
		}
	}
}

func TestTrackConn_ConnID(t *testing.T) {
	p1, _ := pipePair(t)
	base := &mockConnIDConn{Conn: p1, id: 7}
	s := &ServiceState{}

	wrapped := s.TrackConn("tcp", base)
	if wrapped == base {
		t.Fatal("expected wrapped conn, got original")
	}
	if !s.MatchConn("tcp", 7) {
		t.Error("expected MatchConn(tcp, 7) = true")
	}
	if s.MatchConn("tcp", 8) || s.MatchConn("other", 7) {
		t.Error("MatchConn should miss on wrong index/network")
	}

	// 底层不支持半关闭时，基础型包装不能带 CloseRead/CloseWrite，
	// 否则 utils.RelayConns 的接口断言会误判底层能力
	if _, ok := wrapped.(closeReader); ok {
		t.Error("basic wrapper should not expose CloseRead")
	}
	if _, ok := wrapped.(closeWriter); ok {
		t.Error("basic wrapper should not expose CloseWrite")
	}

	if err := wrapped.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}
	if s.MatchConn("tcp", 7) {
		t.Error("expected conn untracked after Close")
	}
	if got := atomic.LoadInt32(&base.closed); got != 1 {
		t.Errorf("underlying Close forwarded %d times, want 1", got)
	}
}

func TestTrackConn_StreamState(t *testing.T) {
	p1, _ := pipePair(t)
	base := &mockStreamConn{Conn: p1, state: &stream.State{Index: 42}}
	s := &ServiceState{}

	wrapped := s.TrackConn("link1", base)
	if wrapped == base {
		t.Fatal("expected wrapped conn, got original")
	}
	if !s.MatchConn("link1", 42) {
		t.Error("expected stream conn tracked by State.Index")
	}

	wrapped.Close()
	if s.MatchConn("link1", 42) {
		t.Error("expected conn untracked after Close")
	}
}

func TestTrackConn_StreamIndexZeroOrNilState(t *testing.T) {
	p1, _ := pipePair(t)
	s := &ServiceState{}

	// Index 为 0 视为无效：不登记，原样返回
	zero := &mockStreamConn{Conn: p1, state: &stream.State{Index: 0}}
	if got := s.TrackConn("link1", zero); got != zero {
		t.Error("Index 0 should be returned as-is")
	}
	// nil state 同样无法提取编号
	nilState := &mockStreamConn{Conn: p1}
	if got := s.TrackConn("link1", nilState); got != nilState {
		t.Error("nil state should be returned as-is")
	}
	if s.MatchConn("link1", 0) {
		t.Error("nothing should be tracked")
	}
}

func TestTrackConn_PlainConnReturnedAsIs(t *testing.T) {
	p1, _ := pipePair(t)
	s := &ServiceState{}

	// 普通 net.Pipe 连接提取不到编号：原样返回，不带包装
	if got := s.TrackConn("tcp", p1); got != p1 {
		t.Error("plain conn should be returned as-is")
	}
	if s.MatchConn("tcp", 1) {
		t.Error("nothing should be tracked for plain conn")
	}
}

func TestTrackConn_EmptyNetwork(t *testing.T) {
	p1, _ := pipePair(t)
	base := &mockConnIDConn{Conn: p1, id: 7}
	s := &ServiceState{}

	if got := s.TrackConn("", base); got != base {
		t.Error("empty network should return conn as-is")
	}
	if s.MatchConn("", 7) {
		t.Error("nothing should be tracked for empty network")
	}
}

func TestTrackConn_CipherConnUnwrap(t *testing.T) {
	p1, p2 := net.Pipe()
	t.Cleanup(func() {
		p1.Close()
		p2.Close()
	})
	inner := &mockConnIDConn{Conn: p1, id: 77}

	// cipherconn.New 内部要做 IV 握手（收发并发），两端需同时构建
	type newResult struct {
		conn net.Conn
		err  error
	}
	peerCh := make(chan newResult, 1)
	go func() {
		cc, err := cipherconn.New(p2, "secret")
		peerCh <- newResult{cc, err}
	}()
	cc, err := cipherconn.New(inner, "secret")
	if err != nil {
		t.Fatalf("cipherconn.New(inner) error = %v", err)
	}
	peer := <-peerCh
	if peer.err != nil {
		t.Fatalf("cipherconn.New(peer) error = %v", peer.err)
	}
	t.Cleanup(func() {
		cc.Close()
		peer.conn.Close()
	})

	s := &ServiceState{}
	wrapped := s.TrackConn("link1", cc)
	if wrapped == cc {
		t.Fatal("expected wrapped conn, got original")
	}
	// 加密包装下的底层连接编号应通过解包提取到
	if !s.MatchConn("link1", 77) {
		t.Error("expected conn tracked through cipherconn layer")
	}
	// cipherconn 不暴露半关闭接口，应为基本型包装
	if _, ok := wrapped.(closeReader); ok {
		t.Error("cipherconn wrapper should not expose CloseRead")
	}

	wrapped.Close()
	if s.MatchConn("link1", 77) {
		t.Error("expected conn untracked after Close")
	}
}

func TestTrackConn_HalfClosePreserved(t *testing.T) {
	p1, _ := pipePair(t)
	base := &mockHalfCloseConn{Conn: p1, id: 9}
	s := &ServiceState{}

	wrapped := s.TrackConn("tcp", base)

	// 底层支持双向半关闭时，包装必须保活这两个接口（utils.RelayConns 依赖）
	cr, ok := wrapped.(closeReader)
	if !ok {
		t.Fatal("wrapped conn should expose CloseRead")
	}
	cw, ok := wrapped.(closeWriter)
	if !ok {
		t.Fatal("wrapped conn should expose CloseWrite")
	}

	if err := cr.CloseRead(); err != nil {
		t.Fatalf("CloseRead error = %v", err)
	}
	if got := atomic.LoadInt32(&base.closeReadCalled); got != 1 {
		t.Errorf("CloseRead forwarded %d times, want 1", got)
	}
	// 仅单向半关闭：连接仍活跃，登记保留
	if !s.MatchConn("tcp", 9) {
		t.Error("conn should stay tracked after single half-close")
	}

	if err := cw.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite error = %v", err)
	}
	if got := atomic.LoadInt32(&base.closeWriteCalled); got != 1 {
		t.Errorf("CloseWrite forwarded %d times, want 1", got)
	}
	// 双向半关闭齐备：注销登记
	if s.MatchConn("tcp", 9) {
		t.Error("conn should be untracked after both halves closed")
	}
}

func TestTrackedConn_ConcurrentCloseAndHalfClose(t *testing.T) {
	p1, _ := pipePair(t)
	base := &mockHalfCloseConn{Conn: p1, id: 5}
	s := &ServiceState{}
	wrapped := s.TrackConn("tcp", base)
	cr := wrapped.(closeReader)
	cw := wrapped.(closeWriter)

	// 模拟 utils.RelayConns 两个 goroutine 并发操作同一连接：
	// Close/CloseRead/CloseWrite 并发下注销必须幂等且无数据竞争
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); wrapped.Close() }()
		go func() { defer wg.Done(); cr.CloseRead() }()
		go func() { defer wg.Done(); cw.CloseWrite() }()
	}
	wg.Wait()

	if s.MatchConn("tcp", 5) {
		t.Error("expected conn untracked after close")
	}
	if got := atomic.LoadInt32(&base.closed); got == 0 {
		t.Error("underlying Close should be forwarded")
	}
}

func TestMatchConn_Untracked(t *testing.T) {
	s := &ServiceState{}
	if s.MatchConn("tcp", 1) {
		t.Error("empty state should not match anything")
	}
}
