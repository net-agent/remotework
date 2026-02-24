package agent

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// mockNetwork 实现 Network 接口，用于单元测试
type mockNetwork struct {
	name string

	dialFn   func(network, addr string) (net.Conn, error)
	listenFn func(network, addr string) (net.Listener, error)
	pingFn   func(domain string, timeout time.Duration) (time.Duration, error)
	meta     *NetworkMeta

	stopped int32
}

func newMockNetwork(name string) *mockNetwork {
	return &mockNetwork{
		name: name,
		pingFn: func(domain string, timeout time.Duration) (time.Duration, error) {
			return time.Millisecond * 10, nil
		},
	}
}

func (m *mockNetwork) GetName() string { return m.name }
func (m *mockNetwork) Dial(network, addr string) (net.Conn, error) {
	if m.dialFn != nil {
		return m.dialFn(network, addr)
	}
	return nil, errors.New("mock: dial not configured")
}
func (m *mockNetwork) Listen(network, addr string) (net.Listener, error) {
	if m.listenFn != nil {
		return m.listenFn(network, addr)
	}
	return nil, errors.New("mock: listen not configured")
}
func (m *mockNetwork) Ping(domain string, timeout time.Duration) (time.Duration, error) {
	if m.pingFn != nil {
		return m.pingFn(domain, timeout)
	}
	return 0, ErrPingNotSupported
}
func (m *mockNetwork) Meta() NetworkMeta {
	if m.meta != nil {
		return *m.meta
	}
	return NetworkMeta{}
}
func (m *mockNetwork) Stop() {
	atomic.StoreInt32(&m.stopped, 1)
}
func (m *mockNetwork) isStopped() bool {
	return atomic.LoadInt32(&m.stopped) == 1
}

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

func (l *mockListener) isClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closed
}

// mockAddr 简单的 net.Addr 实现
type mockAddr string

func (a mockAddr) Network() string { return "mock" }
func (a mockAddr) String() string  { return string(a) }

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
		controller: ctrl,
	}
	return svc, ctrl
}
