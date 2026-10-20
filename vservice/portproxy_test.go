package vservice

import (
	"net"
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
