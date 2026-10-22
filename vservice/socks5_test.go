package vservice

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/net-agent/socks"
)

// mockSocksRequest 实现 socks.Request 接口（requestTarget 单测用）
type mockSocksRequest struct {
	command uint8
	addr    string
}

func (r *mockSocksRequest) GetCommand() uint8           { return r.command }
func (r *mockSocksRequest) GetAddress() (uint8, []byte) { return socks.Domain, []byte(r.addr) }
func (r *mockSocksRequest) GetPort() uint16             { return 443 }
func (r *mockSocksRequest) GetAddrPortStr() string      { return r.addr }

func TestSocks5Controller_RequestTarget(t *testing.T) {
	var gotNetwork, gotAddr string
	dialErr := errors.New("dial failed")
	ctl := &Socks5Controller{
		dialNetwork: "tcp",
		dial: func(network, addr string) (net.Conn, error) {
			gotNetwork, gotAddr = network, addr
			return nil, dialErr
		},
	}

	// CONNECT：拨号经注入的 dialer 发出，网络别名与目标地址透传
	_, err := ctl.requestTarget(&mockSocksRequest{command: socks.ConnectCommand, addr: "example.com:443"}, nil)
	if !errors.Is(err, dialErr) {
		t.Fatalf("requestTarget err = %v, want dial error passthrough %v", err, dialErr)
	}
	if gotNetwork != "tcp" || gotAddr != "example.com:443" {
		t.Errorf("dial args = %q/%q, want tcp/example.com:443", gotNetwork, gotAddr)
	}

	// 非 CONNECT：与库默认实现一致拒绝，不触发拨号
	gotAddr = ""
	_, err = ctl.requestTarget(&mockSocksRequest{command: socks.UDPCommand, addr: "example.com:443"}, nil)
	if !errors.Is(err, socks.ErrReplyCmdNotSupported) {
		t.Fatalf("UDP command err = %v, want %v", err, socks.ErrReplyCmdNotSupported)
	}
	if gotAddr != "" {
		t.Error("non-CONNECT command should not trigger dial")
	}
}

// connIDWrap 给真实连接附加 ConnID，模拟 tcpNetwork 的追踪连接
type connIDWrap struct{ net.Conn }

func (connIDWrap) ConnID() int64 { return 42 }

// TestSocks5Controller_EndToEndTracksTarget 端到端验证：
// 客户端经 socks5 代理访问 echo 服务时，target 侧连接经自定义拨号器发出，
// 且被 TrackConn 登记（MatchConn 命中），连接关闭后注销
func TestSocks5Controller_EndToEndTracksTarget(t *testing.T) {
	// echo 后端
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen failed: %v", err)
	}
	defer echo.Close()
	go func() {
		for {
			conn, err := echo.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				buf := make([]byte, 64)
				for {
					n, err := conn.Read(buf)
					if n > 0 {
						conn.Write(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()

	var gotAddr string
	dial := func(network, addr string) (net.Conn, error) {
		gotAddr = addr
		conn, err := net.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		return connIDWrap{conn}, nil
	}

	lf := &mockListenerFactory{listenFn: func(raw string) (net.Listener, error) {
		// 附加 ConnID，模拟经 tcp 网络插件监听的 listen 侧连接
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		return &idListener{Listener: ln}, nil
	}}
	svc := NewSocks5Service(lf, "sox-e2e", "tcp://127.0.0.1:0", "", "", "tcp", TargetDialer(dial), nil)
	if err := svc.Controller.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer svc.Controller.Close()
	go svc.Controller.Start()

	// 等待 listener 就绪（Init 已同步绑定，直接取地址）
	proxy := &socks.ProxyInfo{Network: "tcp", Address: svc.Controller.(*Socks5Controller).listener.Addr().String()}
	client, err := proxy.Dial(echo.Addr().String())
	if err != nil {
		t.Fatalf("proxy dial failed: %v", err)
	}

	// 拨号目标地址正确；target 侧连接的归属登记由服务端 goroutine 异步完成，轮询等待命中
	if gotAddr != echo.Addr().String() {
		t.Errorf("dial addr = %q, want %q", gotAddr, echo.Addr().String())
	}
	if !waitMatchConn(svc, "tcp", 42, true, 2*time.Second) {
		t.Error("target conn should be tracked (MatchConn hit) while session active")
	}
	if !waitMatchConn(svc, "tcp", 1, true, 2*time.Second) {
		t.Error("listen conn should be tracked (id=1) while session active")
	}

	// pipe 两端（listen 侧 id=1 与 target 侧 id=42）应配对到同一 pipeID
	pipeA, pipeB := waitPipe(svc, "tcp", 1, 2*time.Second), waitPipe(svc, "tcp", 42, 2*time.Second)
	if pipeA == "" || pipeA != pipeB {
		t.Errorf("pipe ends should share one pipeID, got %q / %q", pipeA, pipeB)
	}

	// 收发验证转发通路
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, 8)
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(buf)
	if err != nil || string(buf[:n]) != "ping" {
		t.Fatalf("echo read = %q, %v, want ping", buf[:n], err)
	}

	// 关闭后会话结束，归属登记注销，配对同步清除
	client.Close()
	if !waitMatchConn(svc, "tcp", 42, false, 2*time.Second) ||
		!waitMatchConn(svc, "tcp", 1, false, 2*time.Second) {
		t.Error("both conns should be untracked after session closed")
	}
	if got := svc.MatchPipe("tcp", 1); got != "" || svc.MatchPipe("tcp", 42) != "" {
		t.Errorf("pipe mappings should be removed after session closed, got %q / %q", svc.MatchPipe("tcp", 1), svc.MatchPipe("tcp", 42))
	}
}

// waitMatchConn 轮询等待 MatchConn 达到期望值
func waitMatchConn(svc *Service, network string, index int64, want bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if svc.MatchConn(network, index) == want {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// blockingConn 模拟一条完成 TCP 建连后一直不发送握手数据的连接。
// Read 阻塞到 Close；SetReadDeadline 仅记录，用于断言握手 deadline。
type blockingConn struct {
	closed    chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	deadline  time.Time
}

func newBlockingConn() *blockingConn {
	return &blockingConn{closed: make(chan struct{})}
}

func (c *blockingConn) Read([]byte) (int, error) {
	<-c.closed
	return 0, net.ErrClosed
}

func (c *blockingConn) Write(p []byte) (int, error) {
	select {
	case <-c.closed:
		return 0, net.ErrClosed
	default:
		return len(p), nil
	}
}

func (c *blockingConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func (c *blockingConn) LocalAddr() net.Addr  { return mockAddr("local:0") }
func (c *blockingConn) RemoteAddr() net.Addr { return mockAddr("remote:0") }

func (c *blockingConn) SetDeadline(time.Time) error { return nil }
func (c *blockingConn) SetWriteDeadline(time.Time) error {
	return nil
}
func (c *blockingConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.deadline = t
	c.mu.Unlock()
	return nil
}
func (c *blockingConn) readDeadline() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deadline
}

// TestSocks5Controller_HandshakeDeadlineAndClose 验证：
// Accept 时设置约 5s 的握手读 deadline；握手未完成时 Close 不阻塞，
// 会强关握手中的连接并让 Start 返回。socks 库的 Server.Close 未被调用
// （未 Start 时调用它会 nil panic，已 Start 时会 connWait 阻塞）。
func TestSocks5Controller_HandshakeDeadlineAndClose(t *testing.T) {
	ml := newMockListener("mock:0")
	lf := &mockListenerFactory{listenFn: func(string) (net.Listener, error) { return ml, nil }}
	svc := NewSocks5Service(lf, "sox-close", "tcp://127.0.0.1:0", "", "", "tcp", nil, nil)
	if err := svc.Controller.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	startResult := make(chan error, 1)
	go func() { startResult <- svc.Controller.Start() }()

	conn := newBlockingConn()
	ml.acceptCh <- conn

	// 等待 tracking listener 设置握手 deadline
	var deadline time.Time
	deadlineOK := false
	for i := 0; i < 100; i++ {
		if d := conn.readDeadline(); !d.IsZero() {
			deadline, deadlineOK = d, true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !deadlineOK {
		t.Fatal("accepted conn should get a handshake read deadline")
	}
	if remain := time.Until(deadline); remain < 3*time.Second || remain > 7*time.Second {
		t.Errorf("handshake deadline = %v from now, want ~5s", remain)
	}

	// Close 必须立即返回，不能等待 socks 库的 connWait
	begin := time.Now()
	if err := svc.Controller.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Errorf("Close blocked for %v, want immediate return", elapsed)
	}

	select {
	case <-startResult:
	case <-time.After(2 * time.Second):
		t.Fatal("Start should return after listener/active conns are closed")
	}

	select {
	case <-conn.closed:
	default:
		t.Error("handshake conn should be force-closed by controller Close")
	}

	waitUntil := time.Now().Add(2 * time.Second)
	for time.Now().Before(waitUntil) {
		if svc.GetActiveCount() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("active count = %d after Close, want 0", svc.GetActiveCount())
}

// TestSocks5Controller_CloseWithoutStart 验证未 Start 时 Close 不 panic、幂等。
func TestSocks5Controller_CloseWithoutStart(t *testing.T) {
	ml := newMockListener("mock:0")
	lf := &mockListenerFactory{listenFn: func(string) (net.Listener, error) { return ml, nil }}
	svc := NewSocks5Service(lf, "sox-nostart", "tcp://127.0.0.1:0", "", "", "tcp", nil, nil)
	if err := svc.Controller.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// socks.Server.Close 会在 Run 未启动时解引用 nil listener 而 panic；
	// 此调用不应触发它
	if err := svc.Controller.Close(); err != nil {
		t.Errorf("first Close failed: %v", err)
	}
	if err := svc.Controller.Close(); err != nil {
		t.Errorf("second Close should be idempotent, got %v", err)
	}
	if got := svc.GetStatus(); got == StatusFailed {
		t.Error("normal Close should not mark service failed")
	}
}

// TestSocksTrackedConn_MarkActiveClearsDeadline 验证握手完成后清除读 deadline。
func TestSocksTrackedConn_MarkActiveClearsDeadline(t *testing.T) {
	raw := newBlockingConn()
	tc := &socksTrackedConn{Conn: raw, ctl: &Socks5Controller{state: &ServiceState{}}}
	tc.handshaking.Store(true)
	if err := raw.SetReadDeadline(time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	tc.markActive()
	if got := raw.readDeadline(); !got.IsZero() {
		t.Errorf("read deadline = %v after markActive, want zero", got)
	}
	// 幂等：重复调用不应重新设置 deadline
	tc.markActive()
	if got := raw.readDeadline(); !got.IsZero() {
		t.Errorf("read deadline = %v after second markActive, want zero", got)
	}
}

// TestSocks5Controller_StartPermanentFailureSetsFailed 验证 socks 库 Run 返回
// 不可恢复错误时，controller 永久退出并记录 StatusFailed/LastErr。
func TestSocks5Controller_StartPermanentFailureSetsFailed(t *testing.T) {
	sentinel := errors.New("fatal socks accept failure")
	ln := &fatalAcceptListener{err: sentinel, addr: mockAddr("fatal:0")}
	lf := &mockListenerFactory{listenFn: func(string) (net.Listener, error) { return ln, nil }}
	svc := NewSocks5Service(lf, "sox-fatal", "tcp://127.0.0.1:0", "", "", "tcp", nil, nil)
	if err := svc.Controller.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err := svc.Controller.Start()
	if !errors.Is(err, sentinel) {
		t.Fatalf("Start err = %v, want %v", err, sentinel)
	}
	if got := svc.GetStatus(); got != StatusFailed {
		t.Errorf("status = %v, want StatusFailed", got)
	}
	if got := svc.GetLastErr(); got == "" {
		t.Error("LastErr should be recorded on permanent failure")
	}
}
