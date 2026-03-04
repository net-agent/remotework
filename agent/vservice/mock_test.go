package vservice

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

// mockServiceController 实现 ServiceController 接口
type mockServiceController struct {
	mu       sync.Mutex
	initErr  error
	startErr error
	closeErr error

	initCalled  int32
	startCalled int32
	closeCalled int32

	// startBlock 如果非 nil，Start() 会阻塞直到该 channel 关闭
	startBlock chan struct{}
}

func newMockServiceController() *mockServiceController {
	return &mockServiceController{}
}

func (c *mockServiceController) Init() error {
	atomic.AddInt32(&c.initCalled, 1)
	c.mu.Lock()
	err := c.initErr
	c.mu.Unlock()
	return err
}

func (c *mockServiceController) Start() error {
	atomic.AddInt32(&c.startCalled, 1)
	if c.startBlock != nil {
		<-c.startBlock
	}
	c.mu.Lock()
	err := c.startErr
	c.mu.Unlock()
	return err
}

func (c *mockServiceController) Close() error {
	atomic.AddInt32(&c.closeCalled, 1)
	c.mu.Lock()
	err := c.closeErr
	c.mu.Unlock()
	return err
}

// mockListenerFactory 实现 ListenerFactory 接口
type mockListenerFactory struct {
	listenFn func(raw string) (net.Listener, error)
}

func (f *mockListenerFactory) ListenURL(raw string) (net.Listener, error) {
	if f.listenFn != nil {
		return f.listenFn(raw)
	}
	return newMockListener("mock:0"), nil
}

// mockDialerFactory 实现 DialerFactory 接口
type mockDialerFactory struct {
	dialerFn func(raw string) (QuickDialer, error)
}

func (f *mockDialerFactory) URLDialer(raw string) (QuickDialer, error) {
	if f.dialerFn != nil {
		return f.dialerFn(raw)
	}
	return func() (net.Conn, error) {
		return nil, errors.New("mock: not connected")
	}, nil
}

// mockListener 实现 net.Listener 接口
type mockListener struct {
	mu       sync.Mutex
	closed   bool
	acceptCh chan net.Conn
	addr     net.Addr
}

func newMockListener(addr string) *mockListener {
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

// mockAddr 简单的 net.Addr 实现
type mockAddr string

func (a mockAddr) Network() string { return "mock" }
func (a mockAddr) String() string  { return string(a) }

// newTestService 创建一个用于测试的 Service，使用 mockServiceController
func newTestService(name, typ, listenURL, targetURL string) (*Service, *mockServiceController) {
	ctrl := newMockServiceController()
	svc := &Service{
		ServiceState: ServiceState{
			Type:      typ,
			Name:      name,
			ListenURL: listenURL,
			TargetURL: targetURL,
		},
		Controller: ctrl,
	}
	return svc, ctrl
}
