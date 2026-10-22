package vnet

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

// halfCloseCapable 是 utils.RelayConns 半关闭探测所需的接口形状
// （*net.TCPConn 等支持，net.Pipe 等不支持）
type halfCloseCapable interface {
	CloseRead() error
	CloseWrite() error
}

// trackedConn 包装 net.Conn，累计字节数并维护生命周期状态。
// 基础类型不实现 CloseRead/CloseWrite：RelayConns 的 half-close 探测基于
// 接口断言，基础类型若带上这两个方法会误判底层能力、改变转发行为。
type trackedConn struct {
	net.Conn
	tracker    *connTracker
	id         int32            // 注册表内自增 id，从 1 开始
	direction  stream.Direction // outbound=Dial，inbound=Accept
	localAddr  string           // 注册时捕获的 LocalAddr 文本
	remoteAddr string           // 注册时捕获的 RemoteAddr 文本

	bytesRead    atomic.Int64
	bytesWritten atomic.Int64

	created time.Time

	mu          sync.Mutex
	closed      time.Time
	isClosed    bool
	closeReason string // first-wins：manual/local/remote/stopped
	readClosed  bool
	writeClosed bool
}

// trackedHalfConn 在 trackedConn 基础上暴露 CloseRead/CloseWrite，
// 仅当底层 conn 支持半关闭时才使用此包装（见 connTracker.track 的类型选择）。
type trackedHalfConn struct {
	*trackedConn
	hc halfCloseCapable
}

func (c *trackedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.bytesRead.Add(int64(n))
	}
	if err != nil {
		// 读失败（含 io.EOF）视为对端关闭信号：只归因记录，不改变行为、不主动关连接
		c.presetCloseReason("remote")
	}
	return n, err
}

func (c *trackedConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 {
		c.bytesWritten.Add(int64(n))
	}
	if err != nil {
		c.presetCloseReason("remote")
	}
	return n, err
}

// Close 幂等关闭：首次调用做生命周期 finalize（reason 缺省记 "local"），
// 底层 Close 每次都转发（重复关闭的报错由底层决定）
func (c *trackedConn) Close() error {
	c.finalize("local")
	return c.Conn.Close()
}

// ConnID 返回连接在注册表内的 id（vservice 归属断言的约定接口）
func (c *trackedConn) ConnID() int64 { return int64(c.id) }

// presetCloseReason 预置关闭原因，first-wins（已被占用时不覆盖）
func (c *trackedConn) presetCloseReason(reason string) {
	c.mu.Lock()
	if c.closeReason == "" {
		c.closeReason = reason
	}
	c.mu.Unlock()
}

// finalize 做关闭迁移：标记 isClosed/closed 时间（双向视为已关），
// 从 actives 移到 closeds 环形历史，并触发 OnStreamClosed 回调。
// 重复调用直接返回（回调也因此只触发一次）。
func (c *trackedConn) finalize(reason string) {
	c.mu.Lock()
	if c.isClosed {
		c.mu.Unlock()
		return
	}
	c.isClosed = true
	c.closed = time.Now()
	c.readClosed = true
	c.writeClosed = true
	if c.closeReason == "" {
		c.closeReason = reason
	}
	snapshot := c.snapshotLocked(c.tracker.name)
	c.mu.Unlock()

	c.tracker.moveToClosed(c.id, snapshot)
	if tr := c.tracker.trace; tr != nil && tr.OnStreamClosed != nil {
		tr.OnStreamClosed(snapshot)
	}
}

// snapshotLocked 合成 State 快照，调用方需持有 c.mu
func (c *trackedConn) snapshotLocked(netName string) *stream.State {
	return &stream.State{
		Index:        c.id,
		IsClosed:     c.isClosed,
		Created:      c.created,
		Closed:       c.closed,
		ReadClosed:   c.readClosed,
		WriteClosed:  c.writeClosed,
		CloseReason:  c.closeReason,
		Direction:    c.direction,
		LocalAddr:    stream.NewAddr(netName, c.localAddr),
		RemoteAddr:   stream.NewAddr(netName, c.remoteAddr),
		BytesRead:    c.bytesRead.Load(),
		BytesWritten: c.bytesWritten.Load(),
	}
}

// snapshot 合成 State 快照（锁安全），供 actives 查询使用
func (c *trackedConn) snapshot(netName string) *stream.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotLocked(netName)
}

func (c *trackedHalfConn) CloseRead() error {
	err := c.hc.CloseRead()
	c.mu.Lock()
	c.readClosed = true
	bothClosed := c.writeClosed
	c.mu.Unlock()
	if bothClosed {
		// 双向半关闭即整体关闭：finalize 语义同 Close，reason 缺省记 "local"。
		// 底层 FD 的释放交给后续的 Close（RelayConns 结束时总会调用）。
		c.finalize("local")
	}
	return err
}

func (c *trackedHalfConn) CloseWrite() error {
	err := c.hc.CloseWrite()
	c.mu.Lock()
	c.writeClosed = true
	bothClosed := c.readClosed
	c.mu.Unlock()
	if bothClosed {
		c.finalize("local")
	}
	return err
}

// trackedListener 包装 net.Listener，Accept 成功的连接注册为 inbound
type trackedListener struct {
	net.Listener
	tracker *connTracker
}

func (l *trackedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return l.tracker.track(conn, stream.DirectionInbound), nil
}

// connTracker 是 tcpNetwork 的连接注册表：actives 全量 + closeds 环形历史
// （容量复用 maxClosedStreamHist，超出淘汰最旧）。无后台 goroutine。
// trace 非 nil 时在连接注册/完全关闭时触发流生命周期回调（锁外调用）。
type connTracker struct {
	name   string // tcpNetwork 的 name，用于合成 stream.Addr 的 network 字段
	trace  *StreamTrace
	nextID atomic.Int32

	mu      sync.Mutex
	actives map[int32]*trackedConn
	closeds stateRing // 关闭时刻的 State 快照，固定容量环形历史
}

func newConnTracker(name string, trace *StreamTrace) *connTracker {
	return &connTracker{
		name:    name,
		trace:   trace,
		actives: make(map[int32]*trackedConn),
	}
}

// track 注册一条连接并返回包装后的 net.Conn。
// 底层支持半关闭时返回 trackedHalfConn（保活 CloseRead/CloseWrite 断言），
// 否则返回基础 trackedConn。注册成功后触发 OnStreamOpen 回调
// （锁外调用，此刻快照字节数为零）。
func (t *connTracker) track(conn net.Conn, direction stream.Direction) net.Conn {
	c := &trackedConn{
		Conn:       conn,
		tracker:    t,
		id:         t.nextID.Add(1),
		direction:  direction,
		localAddr:  addrText(conn.LocalAddr()),
		remoteAddr: addrText(conn.RemoteAddr()),
		created:    time.Now(),
	}
	t.mu.Lock()
	t.actives[c.id] = c
	t.mu.Unlock()

	if t.trace != nil && t.trace.OnStreamOpen != nil {
		t.trace.OnStreamOpen(c.snapshot(t.name))
	}

	if hc, ok := conn.(halfCloseCapable); ok {
		return &trackedHalfConn{trackedConn: c, hc: hc}
	}
	return c
}

// moveToClosed 把连接从 actives 迁移到 closeds 环形历史（超出容量淘汰最旧）
func (t *connTracker) moveToClosed(id int32, snapshot *stream.State) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.actives, id)
	t.closeds.push(snapshot, maxClosedStreamHist)
}

// CloseAll 关闭所有活跃连接（Stop 时调用），reason 预置（first-wins 不覆盖已有归因）
func (t *connTracker) CloseAll(reason string) {
	t.mu.Lock()
	conns := make([]*trackedConn, 0, len(t.actives))
	for _, c := range t.actives {
		conns = append(conns, c)
	}
	t.mu.Unlock()

	for _, c := range conns {
		c.presetCloseReason(reason)
		c.Close()
	}
}

// GetStreamStates 返回 (actives, closeds) 快照，语义对齐 flex 侧：
// actives 为未完全关闭的连接（含半关闭），closeds 为关闭历史。
func (t *connTracker) GetStreamStates() (actives, closeds []*stream.State) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.actives) > 0 {
		actives = make([]*stream.State, 0, len(t.actives))
		for _, c := range t.actives {
			actives = append(actives, c.snapshot(t.name))
		}
	}
	closeds = t.closeds.snapshot()
	return actives, closeds
}

// CloseStream 按 State.Index 手动关闭一条活跃连接（reason 记 "manual"）。
// 找不到（或已迁移出 actives）返回 false。
func (t *connTracker) CloseStream(index int32) bool {
	t.mu.Lock()
	c, ok := t.actives[index]
	t.mu.Unlock()
	if !ok {
		return false
	}
	c.presetCloseReason("manual")
	c.Close()
	return true
}

func addrText(a net.Addr) string {
	if a == nil {
		return ""
	}
	return a.String()
}
