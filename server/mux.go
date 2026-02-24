package server

import (
	"bufio"
	"log/slog"
	"net"
)

// Mux 单端口多协议分流器
// 通过 peek 首字节区分 flex 二进制协议（< 0x20）和 HTTP 文本协议
type Mux struct {
	listener net.Listener
	flex     *connChan
	http     *connChan
	logger   *slog.Logger
}

func NewMux(l net.Listener, logger *slog.Logger) *Mux {
	addr := l.Addr()
	return &Mux{
		listener: l,
		flex:     newConnChan(addr),
		http:     newConnChan(addr),
		logger:   logger,
	}
}

func (m *Mux) FlexListener() net.Listener { return m.flex }
func (m *Mux) HTTPListener() net.Listener { return m.http }

// Serve 阻塞式运行，accept 连接并按协议分流
func (m *Mux) Serve() error {
	defer m.flex.Close()
	defer m.http.Close()
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			return err
		}
		go m.handle(conn)
	}
}

func (m *Mux) Close() error {
	return m.listener.Close()
}

func (m *Mux) handle(conn net.Conn) {
	br := bufio.NewReaderSize(conn, 4096)
	peek, err := br.Peek(1)
	if err != nil {
		m.logger.Warn("peek failed", "remote", conn.RemoteAddr(), "error", err)
		conn.Close()
		return
	}

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

// connChan 通过 channel 实现 net.Listener
type connChan struct {
	ch   chan net.Conn
	done chan struct{}
	addr net.Addr
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
	case l.ch <- conn:
	case <-l.done:
		conn.Close()
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
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	return nil
}

func (l *connChan) Addr() net.Addr { return l.addr }
