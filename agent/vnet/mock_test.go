package vnet

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// MockNetwork 实现 Network 接口，用于单元测试
type MockNetwork struct {
	name string

	dialFn   func(network, addr string) (net.Conn, error)
	listenFn func(network, addr string) (net.Listener, error)
	pingFn   func(domain string, timeout time.Duration) (time.Duration, error)
	meta     *NetworkMeta

	stopped int32
}

func NewMockNetwork(name string) *MockNetwork {
	return &MockNetwork{
		name: name,
		pingFn: func(domain string, timeout time.Duration) (time.Duration, error) {
			return time.Millisecond * 10, nil
		},
	}
}
func (mn *MockNetwork) SetListenFn(fn func(network, addr string) (net.Listener, error)) {
	mn.listenFn = fn
}
func (mn *MockNetwork) SetDialFn(fn func(network, addr string) (net.Conn, error)) {
	mn.dialFn = fn
}

func (m *MockNetwork) GetName() string { return m.name }
func (m *MockNetwork) Dial(network, addr string) (net.Conn, error) {
	if m.dialFn != nil {
		return m.dialFn(network, addr)
	}
	return nil, errors.New("mock: dial not configured")
}
func (m *MockNetwork) Listen(network, addr string) (net.Listener, error) {
	if m.listenFn != nil {
		return m.listenFn(network, addr)
	}
	return nil, errors.New("mock: listen not configured")
}
func (m *MockNetwork) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	if m.pingFn != nil {
		return m.pingFn(domain, timeout)
	}
	return 0, ErrPingNotSupported
}
func (m *MockNetwork) Meta() NetworkMeta {
	if m.meta != nil {
		return *m.meta
	}
	return NetworkMeta{}
}
func (m *MockNetwork) Stop() {
	atomic.StoreInt32(&m.stopped, 1)
}
func (m *MockNetwork) isStopped() bool {
	return atomic.LoadInt32(&m.stopped) == 1
}

// mockListener 实现 net.Listener 接口
type mockListener struct {
	mu       sync.Mutex
	closed   bool
	acceptCh chan net.Conn
	addr     net.Addr
}

func NewMockListener(addr string) *mockListener {
	return &mockListener{
		acceptCh: make(chan net.Conn),
		addr:     mockAddr(addr),
	}
}

func (l *mockListener) Accept() (net.Conn, error) {
	conn, ok := <-l.acceptCh
	if !ok {
		return nil, errors.New("listener closed")
	}
	return conn, nil
}

func (l *mockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return errors.New("already closed")
	}
	l.closed = true
	close(l.acceptCh)
	return nil
}

func (l *mockListener) Addr() net.Addr {
	return l.addr
}

func (l *mockListener) isClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closed
}

// mockAddr 简单的 net.Addr 实现
type mockAddr string

func (a mockAddr) Network() string { return "mock" }
func (a mockAddr) String() string  { return string(a) }
