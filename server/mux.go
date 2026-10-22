package server

import (
	"bufio"
	"log/slog"
	"net"
	"sync"
	"time"
)

// 分流前的首字节探测超时：连接建立后必须尽快表明协议，
// 避免未认证的慢速连接长期占用 goroutine/fd。
const peekTimeout = 5 * time.Second

// muxMaxPending 是分流前同时处理的连接上限。首字节探测/协议分流阶段
// 尚未进入 flex 的准入控制，必须在 mux 层兜住慢速连接洪泛。
const muxMaxPending = 4096

// Mux 单端口多协议分流器
// 通过 peek 首字节区分 flex 二进制协议（< 0x20）和 HTTP 文本协议
type Mux struct {
	listener net.Listener
	flex     *connChan
	http     *connChan
	logger   *slog.Logger
	slots    chan struct{}
}

func NewMux(l net.Listener, logger *slog.Logger) *Mux {
	addr := l.Addr()
	return &Mux{
		listener: l,
		flex:     newConnChan(addr),
		http:     newConnChan(addr),
		logger:   logger,
		slots:    make(chan struct{}, muxMaxPending),
	}
}

func (m *Mux) FlexListener() net.Listener { return m.flex }
func (m *Mux) HTTPListener() net.Listener { return m.http }

// CloseHTTP 关闭 HTTP 分流通道。WsEnable=false 时使用，避免无人消费的
// HTTP 连接把分流 goroutine 永久阻塞在 Offer 上。
func (m *Mux) CloseHTTP() { _ = m.http.Close() }

// Serve 阻塞式运行，accept 连接并按协议分流
func (m *Mux) Serve() error {
	defer m.flex.Close()
	defer m.http.Close()
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			return err
		}
		select {
		case m.slots <- struct{}{}:
		default:
			// 分流阶段已达上限：快速拒绝，避免 goroutine/fd 无界增长。
			m.logger.Warn("too many pending connections, rejecting", "remote", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}
		go func(c net.Conn) {
			defer func() { <-m.slots }()
			m.handle(c)
		}(conn)
	}
}

func (m *Mux) Close() error {
	_ = m.flex.Close()
	_ = m.http.Close()
	return m.listener.Close()
}

func (m *Mux) handle(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(30 * time.Second)
	}

	// 首字节探测必须有超时；否则客户端连上后不发数据即可长期占用资源。
	_ = conn.SetReadDeadline(time.Now().Add(peekTimeout))
	br := bufio.NewReaderSize(conn, 4096)
	peek, err := br.Peek(1)
	if err != nil {
		m.logger.Warn("peek failed", "remote", conn.RemoteAddr(), "error", err)
		_ = conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	pc := &peekConn{Conn: conn, reader: br}

	if peek[0] < 0x20 {
		m.flex.Offer(pc)
	} else {
		m.http.Offer(pc)
	}
}

// peekConn 包装 net.Conn，使 peek 过的字节可以被后续正常读取
type peekConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *peekConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// connChan 通过 channel 实现 net.Listener。
// Close 可重复调用；Offer 与 Close 并发时不会把连接投递到无人消费的通道。
type connChan struct {
	ch        chan net.Conn
	done      chan struct{}
	closeOnce sync.Once
	addr      net.Addr
}

func newConnChan(addr net.Addr) *connChan {
	return &connChan{
		ch:   make(chan net.Conn, 64),
		done: make(chan struct{}),
		addr: addr,
	}
}

func (l *connChan) Offer(conn net.Conn) {
	select {
	case <-l.done:
		_ = conn.Close()
		return
	default:
	}

	select {
	case l.ch <- conn:
		// Close 可能恰好在投递后发生；此时没有消费者，主动关闭连接。
		select {
		case <-l.done:
			_ = conn.Close()
		default:
		}
	case <-l.done:
		_ = conn.Close()
	}
}

func (l *connChan) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *connChan) Close() error {
	l.closeOnce.Do(func() { close(l.done) })
	return nil
}

func (l *connChan) Addr() net.Addr { return l.addr }
