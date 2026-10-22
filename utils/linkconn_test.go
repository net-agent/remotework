package utils

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type halfCloseConn struct {
	read      *io.PipeReader
	write     *io.PipeWriter
	closeOnce sync.Once
}

func (c *halfCloseConn) Read(p []byte) (int, error) {
	return c.read.Read(p)
}

func (c *halfCloseConn) Write(p []byte) (int, error) {
	return c.write.Write(p)
}

func (c *halfCloseConn) Close() error {
	c.closeOnce.Do(func() {
		_ = c.write.Close()
		_ = c.read.Close()
	})
	return nil
}

func (c *halfCloseConn) CloseWrite() error {
	return c.write.Close()
}

func (c *halfCloseConn) CloseRead() error {
	return c.read.Close()
}

func newHalfClosePair() (*halfCloseConn, *halfCloseConn) {
	leftToRightReader, leftToRightWriter := io.Pipe()
	rightToLeftReader, rightToLeftWriter := io.Pipe()

	left := &halfCloseConn{
		read:  rightToLeftReader,
		write: leftToRightWriter,
	}
	right := &halfCloseConn{
		read:  leftToRightReader,
		write: rightToLeftWriter,
	}

	return left, right
}

type linkResult struct {
	bytesToA int64
	bytesToB int64
	err      error
}

type failWriterConn struct {
	io.ReadWriteCloser
	writeErr error
}

func (c *failWriterConn) Write(p []byte) (int, error) {
	return 0, c.writeErr
}

func (c *failWriterConn) CloseWrite() error {
	if cw, ok := c.ReadWriteCloser.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.ReadWriteCloser.Close()
}

func (c *failWriterConn) CloseRead() error {
	if cr, ok := c.ReadWriteCloser.(interface{ CloseRead() error }); ok {
		return cr.CloseRead()
	}
	return nil
}

func TestLinkReadWriteCloser_AllowsHalfCloseResponseFlow(t *testing.T) {
	client, linkA := newHalfClosePair()
	linkB, server := newHalfClosePair()
	defer client.Close()
	defer server.Close()

	resultCh := make(chan linkResult, 1)
	go func() {
		bytesToA, bytesToB, err := RelayConns(linkA, linkB)
		resultCh <- linkResult{bytesToA: bytesToA, bytesToB: bytesToB, err: err}
	}()

	request := []byte("GET /resource HTTP/1.1\r\nHost: example.test\r\n\r\n")
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello")

	serverErrCh := make(chan error, 1)
	go func() {
		defer close(serverErrCh)
		reqBytes, err := io.ReadAll(server)
		if err != nil {
			serverErrCh <- err
			return
		}
		if !bytes.Equal(reqBytes, request) {
			serverErrCh <- errors.New("request mismatch")
			return
		}
		if _, err = server.Write(response); err != nil {
			serverErrCh <- err
			return
		}
		serverErrCh <- server.CloseWrite()
	}()

	if _, err := client.Write(request); err != nil {
		t.Fatalf("write request failed: %v", err)
	}
	if err := client.CloseWrite(); err != nil {
		t.Fatalf("client close write failed: %v", err)
	}

	clientReadCh := make(chan struct {
		data []byte
		err  error
	}, 1)
	go func() {
		data, err := io.ReadAll(client)
		clientReadCh <- struct {
			data []byte
			err  error
		}{data: data, err: err}
	}()

	select {
	case serverErr := <-serverErrCh:
		if serverErr != nil {
			t.Fatalf("server side failed: %v", serverErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server side timeout")
	}

	var clientRead struct {
		data []byte
		err  error
	}
	select {
	case clientRead = <-clientReadCh:
	case <-time.After(2 * time.Second):
		t.Fatal("client read timeout")
	}
	if clientRead.err != nil {
		t.Fatalf("client read failed: %v", clientRead.err)
	}
	if !bytes.Equal(clientRead.data, response) {
		t.Fatalf("response mismatch: got=%q want=%q", string(clientRead.data), string(response))
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("link returned unexpected error: %v", result.err)
		}
		if result.bytesToA != int64(len(response)) {
			t.Fatalf("bytesToA mismatch: got=%d want=%d", result.bytesToA, len(response))
		}
		if result.bytesToB != int64(len(request)) {
			t.Fatalf("bytesToB mismatch: got=%d want=%d", result.bytesToB, len(request))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("link result timeout")
	}
}

func TestLinkReadWriteCloser_ReportsFirstErrorAndBytes(t *testing.T) {
	client, linkA := newHalfClosePair()
	linkBRaw, server := newHalfClosePair()
	defer client.Close()
	defer server.Close()

	sentinel := errors.New("forced write error")
	linkB := &failWriterConn{ReadWriteCloser: linkBRaw, writeErr: sentinel}

	resultCh := make(chan linkResult, 1)
	go func() {
		bytesToA, bytesToB, err := RelayConns(linkA, linkB)
		resultCh <- linkResult{bytesToA: bytesToA, bytesToB: bytesToB, err: err}
	}()

	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatalf("client write failed: %v", err)
	}
	if err := client.CloseWrite(); err != nil {
		t.Fatalf("client close write failed: %v", err)
	}

	select {
	case result := <-resultCh:
		if !errors.Is(result.err, sentinel) {
			t.Fatalf("unexpected error: got=%v want=%v", result.err, sentinel)
		}
		if result.bytesToB != 0 {
			t.Fatalf("unexpected bytesToB: got=%d want=0", result.bytesToB)
		}
		if result.bytesToA != 0 {
			t.Fatalf("unexpected bytesToA: got=%d want=0", result.bytesToA)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("link result timeout")
	}
}

// TestRelayConns_LargePayloadBufferReuse 用远超 32KB 池化缓冲区的数据做多轮双向
// 转发，验证 buffer 从 sync.Pool 取用/归还后数据仍完整、字节计数正确。
func TestRelayConns_LargePayloadBufferReuse(t *testing.T) {
	request := bytes.Repeat([]byte("request-"), 12*1024)   // ~96KB
	response := bytes.Repeat([]byte("response-"), 10*1024) // ~90KB
	if len(request) <= relayBufferSize || len(response) <= relayBufferSize {
		t.Fatalf("test payload must exceed pool buffer (%d bytes)", relayBufferSize)
	}

	for round := 0; round < 3; round++ {
		client, linkA := newHalfClosePair()
		linkB, server := newHalfClosePair()

		resultCh := make(chan linkResult, 1)
		go func() {
			bytesToA, bytesToB, err := RelayConns(linkA, linkB)
			resultCh <- linkResult{bytesToA: bytesToA, bytesToB: bytesToB, err: err}
		}()

		serverErrCh := make(chan error, 1)
		go func() {
			got := make([]byte, len(request))
			if _, err := io.ReadFull(server, got); err != nil {
				serverErrCh <- err
				return
			}
			if !bytes.Equal(got, request) {
				serverErrCh <- errors.New("request payload mismatch")
				return
			}
			if _, err := server.Write(response); err != nil {
				serverErrCh <- err
				return
			}
			serverErrCh <- server.CloseWrite()
		}()

		if _, err := client.Write(request); err != nil {
			t.Fatalf("round %d: client write failed: %v", round, err)
		}
		if err := client.CloseWrite(); err != nil {
			t.Fatalf("round %d: client CloseWrite failed: %v", round, err)
		}

		clientDataCh := make(chan struct {
			data []byte
			err  error
		}, 1)
		go func() {
			data, err := io.ReadAll(client)
			clientDataCh <- struct {
				data []byte
				err  error
			}{data: data, err: err}
		}()

		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Fatalf("round %d: server failed: %v", round, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: server timeout", round)
		}

		var clientData struct {
			data []byte
			err  error
		}
		select {
		case clientData = <-clientDataCh:
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: client read timeout", round)
		}
		if clientData.err != nil {
			t.Fatalf("round %d: client read failed: %v", round, clientData.err)
		}
		if !bytes.Equal(clientData.data, response) {
			t.Fatalf("round %d: response payload mismatch (got %d bytes, want %d)", round, len(clientData.data), len(response))
		}

		select {
		case result := <-resultCh:
			if result.err != nil {
				t.Fatalf("round %d: relay error: %v", round, result.err)
			}
			if result.bytesToA != int64(len(response)) {
				t.Errorf("round %d: bytesToA = %d, want %d", round, result.bytesToA, len(response))
			}
			if result.bytesToB != int64(len(request)) {
				t.Errorf("round %d: bytesToB = %d, want %d", round, result.bytesToB, len(request))
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: relay result timeout", round)
		}
	}
}
