package vservice

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// idListener 包装真实 listener，Accept 出的连接附加自增 ConnID，
// 模拟 tcpNetwork trackedListener 产出的可追踪连接
type idListener struct {
	net.Listener
	nextID int64
}

func (l *idListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &mockConnIDConn{Conn: c, id: atomic.AddInt64(&l.nextID, 1)}, nil
}

// startEchoServer 启动一个 TCP echo 服务，随测试结束关闭
func startEchoServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen failed: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
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
	return ln
}

// TestPortproxyController_PipePairing 端到端验证：
// portproxy 建立 pipe 后，listen 侧与 target 侧连接配对到同一 pipeID
// （serve 内 TrackConn 之后、RelayConns 之前的 LinkConns 调用点），
// 连接关闭后配对随归属登记一并注销
func TestPortproxyController_PipePairing(t *testing.T) {
	echo := startEchoServer(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	lf := &mockListenerFactory{listenFn: func(raw string) (net.Listener, error) {
		return &idListener{Listener: ln}, nil
	}}
	df := &mockDialerFactory{dialerFn: func(raw string) (QuickDialer, error) {
		return func() (net.Conn, error) {
			c, err := net.Dial("tcp", echo.Addr().String())
			if err != nil {
				return nil, err
			}
			return &mockConnIDConn{Conn: c, id: 1001}, nil
		}, nil
	}}

	svc := NewPortproxyService(lf, df, "pp-pipe", "tcp://127.0.0.1:0", "tcp://"+echo.Addr().String(), nil)
	if err := svc.Controller.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer svc.Controller.Close()
	go svc.Controller.Start()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}

	// pipe 建立是异步的：等两端归属登记命中后再断言配对
	if !waitMatchConn(svc, "tcp", 1, true, 2*time.Second) {
		t.Fatal("listen-side conn should be tracked (id=1)")
	}
	if !waitMatchConn(svc, "tcp", 1001, true, 2*time.Second) {
		t.Fatal("target-side conn should be tracked (id=1001)")
	}

	pipeA := waitPipe(svc, "tcp", 1, 2*time.Second)
	pipeB := waitPipe(svc, "tcp", 1001, 2*time.Second)
	if pipeA == "" || pipeB == "" {
		t.Fatalf("both pipe ends should be paired, got %q / %q", pipeA, pipeB)
	}
	if pipeA != pipeB {
		t.Errorf("pipe ends should share one pipeID, got %q / %q", pipeA, pipeB)
	}
	if pipeA != "pp-pipe#1" {
		t.Errorf("pipeID = %q, want %q", pipeA, "pp-pipe#1")
	}

	// 转发通路验证
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, 8)
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, err := client.Read(buf); err != nil || string(buf[:n]) != "ping" {
		t.Fatalf("echo read = %q, %v, want ping", buf[:n], err)
	}

	// 关闭后 pipe 两端登记注销，配对同步清除
	client.Close()
	if !waitMatchConn(svc, "tcp", 1, false, 2*time.Second) ||
		!waitMatchConn(svc, "tcp", 1001, false, 2*time.Second) {
		t.Error("both conns should be untracked after session closed")
	}
	if got := svc.MatchPipe("tcp", 1); got != "" {
		t.Errorf("listen-side pipe mapping should be removed, got %q", got)
	}
	if got := svc.MatchPipe("tcp", 1001); got != "" {
		t.Errorf("target-side pipe mapping should be removed, got %q", got)
	}
}

// newPortproxyTestService 构造一个监听 127.0.0.1 随机端口、目标为 echo 的 portproxy 服务
func newPortproxyTestService(t *testing.T, ln net.Listener, lf ListenerFactory, echo net.Listener) *Service {
	t.Helper()
	df := &mockDialerFactory{dialerFn: func(raw string) (QuickDialer, error) {
		return func() (net.Conn, error) {
			c, err := net.Dial("tcp", echo.Addr().String())
			if err != nil {
				return nil, err
			}
			return &mockConnIDConn{Conn: c, id: 1001}, nil
		}, nil
	}}
	svc := NewPortproxyService(lf, df, "pp-lifecycle", "tcp://127.0.0.1:0", "tcp://"+echo.Addr().String(), nil)
	if err := svc.Controller.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	return svc
}

func waitActiveCount(svc *Service, want int32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if svc.GetActiveCount() == want {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestPortproxyController_CloseKillsActiveConns 验证 Close 会强杀已建立的活跃连接，
// 而不是只关闭 listener；同时 Close 幂等、计数最终归零。
func TestPortproxyController_CloseKillsActiveConns(t *testing.T) {
	echo := startEchoServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	lf := &mockListenerFactory{listenFn: func(string) (net.Listener, error) {
		return &idListener{Listener: ln}, nil
	}}
	svc := newPortproxyTestService(t, ln, lf, echo)
	go svc.Controller.Start()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}
	defer client.Close()

	// 先确认转发通路已建立（连接处于活跃状态）
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, 8)
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, err := client.Read(buf); err != nil || string(buf[:n]) != "ping" {
		t.Fatalf("echo read = %q, %v, want ping", buf[:n], err)
	}
	if !waitActiveCount(svc, 1, 2*time.Second) {
		t.Fatalf("active count = %d, want 1", svc.GetActiveCount())
	}

	if err := svc.Controller.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 活跃连接被主动关闭：客户端读取应立即失败而不是继续阻塞
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Read(buf); err == nil {
		t.Error("client read should fail after controller Close killed active conn")
	}
	if !waitActiveCount(svc, 0, 2*time.Second) {
		t.Errorf("active count = %d after Close, want 0", svc.GetActiveCount())
	}

	// 第二次 Close 幂等（tunnel 的 Drain 与 cleanup 可能各调用一次）
	if err := svc.Controller.Close(); err != nil {
		t.Errorf("second Close should be idempotent, got %v", err)
	}
}

// temporaryAcceptError 模拟可恢复的 Accept 错误
type temporaryAcceptError struct{}

func (temporaryAcceptError) Error() string   { return "temporary accept error" }
func (temporaryAcceptError) Timeout() bool   { return false }
func (temporaryAcceptError) Temporary() bool { return true }

// flakyListener 前 fails 次 Accept 返回临时错误，之后透传真实 listener
type flakyListener struct {
	net.Listener
	mu    sync.Mutex
	fails int
}

func (l *flakyListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.fails > 0 {
		l.fails--
		l.mu.Unlock()
		return nil, temporaryAcceptError{}
	}
	l.mu.Unlock()
	return l.Listener.Accept()
}

// TestPortproxyController_StartRetriesTemporaryAcceptError 验证临时 Accept 错误
// 会被指数退避重试而不是让服务永久退出。
func TestPortproxyController_StartRetriesTemporaryAcceptError(t *testing.T) {
	echo := startEchoServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	flaky := &flakyListener{Listener: ln, fails: 3}
	lf := &mockListenerFactory{listenFn: func(string) (net.Listener, error) { return flaky, nil }}
	svc := newPortproxyTestService(t, ln, lf, echo)
	defer svc.Controller.Close()
	go svc.Controller.Start()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}
	defer client.Close()

	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, 8)
	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	if n, err := client.Read(buf); err != nil || string(buf[:n]) != "ping" {
		t.Fatalf("echo read = %q, %v, want ping (accept retry should recover)", buf[:n], err)
	}
}

// fatalAcceptListener 每次 Accept 都返回不可恢复错误
type fatalAcceptListener struct {
	err  error
	addr net.Addr
}

func (l *fatalAcceptListener) Accept() (net.Conn, error) { return nil, l.err }
func (l *fatalAcceptListener) Close() error              { return nil }
func (l *fatalAcceptListener) Addr() net.Addr            { return l.addr }

// TestPortproxyController_StartPermanentFailureSetsFailed 验证不可恢复错误
// 会让 controller 永久退出，并记录 StatusFailed/LastErr。
func TestPortproxyController_StartPermanentFailureSetsFailed(t *testing.T) {
	sentinel := errors.New("fatal accept failure")
	ln := &fatalAcceptListener{err: sentinel, addr: mockAddr("fatal:0")}
	lf := &mockListenerFactory{listenFn: func(string) (net.Listener, error) { return ln, nil }}
	df := &mockDialerFactory{}
	svc := NewPortproxyService(lf, df, "pp-fatal", "tcp://127.0.0.1:0", "tcp://127.0.0.1:1", nil)
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
