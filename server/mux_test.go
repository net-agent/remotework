package server

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

var testLogger = slog.Default()

func TestMux_FlexRouting(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	var got []byte
	var wg sync.WaitGroup
	wg.Add(1)

	m := NewMux(l, testLogger)
	go func() {
		conn, err := m.FlexListener().Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 16)
		n, _ := conn.Read(buf)
		got = buf[:n]
		wg.Done()
	}()
	go m.Serve()

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte{0x01, 0x02, 0x03})
	conn.Close()

	wg.Wait()
	if len(got) < 1 || got[0] != 0x01 {
		t.Fatalf("expected flex to receive [0x01 ...], got %v", got)
	}
}

func TestMux_HTTPRouting(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	m := NewMux(l, testLogger)
	go http.Serve(m.HTTPListener(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	go m.Serve()

	resp, err := http.Get("http://" + l.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("expected 'ok', got %q", string(body))
	}
}

func TestMux_PeekFailure(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	m := NewMux(l, testLogger)
	go m.Serve()

	// 连接后立即关闭，peek 会失败
	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	time.Sleep(50 * time.Millisecond)
}

func TestMux_Close(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	m := NewMux(l, testLogger)
	done := make(chan error, 1)
	go func() {
		done <- m.Serve()
	}()

	m.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from Serve after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after Close")
	}
}

func TestMux_FlexByteBoundary(t *testing.T) {
	for _, b := range []byte{0x00, 0x0A, 0x1F} {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}

		var flexCalled bool
		var wg sync.WaitGroup
		wg.Add(1)

		m := NewMux(l, testLogger)
		go func() {
			conn, err := m.FlexListener().Accept()
			if err != nil {
				return
			}
			flexCalled = true
			conn.Close()
			wg.Done()
		}()
		go m.Serve()

		conn, err := net.Dial("tcp", l.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		conn.Write([]byte{b})
		wg.Wait()
		conn.Close()
		l.Close()

		if !flexCalled {
			t.Fatalf("byte 0x%02X should route to flex", b)
		}
	}
}

func TestConnChanOfferAfterClose(t *testing.T) {
	l := newConnChan(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	a, b := net.Pipe()
	defer b.Close()
	l.Offer(a)

	_ = a.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := a.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection offered after Close should be closed")
	}
}

func TestMux_HTTPDisabledClosesConnection(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	m := NewMux(l, testLogger)
	m.CloseHTTP()
	go m.Serve()

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: test\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("HTTP connection should be closed when WsEnable=false")
	}
}
