package utils

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/net-agent/cipherconn"
)

// TestSecretListener_EncryptedRoundTrip 验证包装后的 listener 能与客户端
// 完成 cipherconn 握手，并且双向数据（走加密层）保持一致。
func TestSecretListener_EncryptedRoundTrip(t *testing.T) {
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	sl := NewSecretListener(raw, "test-secret")
	defer sl.Close()

	clientRaw, err := net.Dial("tcp", raw.Addr().String())
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}
	defer clientRaw.Close()
	_ = clientRaw.SetDeadline(time.Now().Add(5 * time.Second))
	client, err := cipherconn.New(clientRaw, "test-secret")
	if err != nil {
		t.Fatalf("client handshake failed: %v", err)
	}
	_ = clientRaw.SetDeadline(time.Time{})

	type acceptResult struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		conn, err := sl.Accept()
		accepted <- acceptResult{conn: conn, err: err}
	}()

	var server net.Conn
	select {
	case res := <-accepted:
		if res.err != nil {
			t.Fatalf("Accept failed: %v", res.err)
		}
		server = res.conn
	case <-time.After(5 * time.Second):
		t.Fatal("Accept timeout")
	}
	defer server.Close()

	// client -> server
	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatalf("client write failed: %v", err)
	}
	buf := make([]byte, 16)
	server.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := server.Read(buf)
	if err != nil || string(buf[:n]) != "hello" {
		t.Fatalf("server read = %q, %v, want hello", buf[:n], err)
	}

	// server -> client
	if _, err := server.Write([]byte("world")); err != nil {
		t.Fatalf("server write failed: %v", err)
	}
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = client.Read(buf)
	if err != nil || string(buf[:n]) != "world" {
		t.Fatalf("client read = %q, %v, want world", buf[:n], err)
	}
}

// TestSecretListener_CloseUnblocksAccept 验证 Close 后阻塞中的 Accept 立即返回。
func TestSecretListener_CloseUnblocksAccept(t *testing.T) {
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	sl := NewSecretListener(raw, "secret")

	acceptErr := make(chan error, 1)
	go func() {
		_, err := sl.Accept()
		acceptErr <- err
	}()
	time.Sleep(20 * time.Millisecond) // 让 Accept 进入阻塞

	begin := time.Now()
	if err := sl.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Errorf("Close blocked for %v, want immediate return", elapsed)
	}

	select {
	case err := <-acceptErr:
		if !errors.Is(err, net.ErrClosed) {
			t.Errorf("Accept err = %v, want net.ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept still blocked after Close")
	}

	// 幂等：第二次 Close 返回同一结果
	if err := sl.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

// TestSecretListener_CloseDuringHandshake 验证握手未完成时 Close 也会快速
// 关闭握手中的连接，不等待 10s 握手超时。
func TestSecretListener_CloseDuringHandshake(t *testing.T) {
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	sl := NewSecretListener(raw, "secret")
	defer sl.Close()

	client, err := net.Dial("tcp", raw.Addr().String())
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}
	defer client.Close()
	// 客户端不参与握手：服务端 cipherconn.New 会阻塞在读取 IV 上

	acceptErr := make(chan error, 1)
	go func() {
		_, err := sl.Accept()
		acceptErr <- err
	}()
	time.Sleep(100 * time.Millisecond) // 等 accept 循环登记握手连接

	begin := time.Now()
	if err := sl.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Errorf("Close during handshake blocked for %v, want immediate return", elapsed)
	}

	select {
	case err := <-acceptErr:
		if !errors.Is(err, net.ErrClosed) {
			t.Errorf("Accept err = %v, want net.ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept still blocked after Close during handshake")
	}
}
