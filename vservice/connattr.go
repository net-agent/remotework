package vservice

import (
	"net"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/net-agent/flex/v3/stream"
)

// connRef 标识一条网络连接：网络别名 + 连接在该网络内的编号
type connRef struct {
	Network string
	Index   int64
}

// connUnwrapDepth 限制加密层解包深度，防止异常嵌套结构导致死循环
const connUnwrapDepth = 4

// connIDer 对应 tcp 网络追踪连接的结构化接口（无需 import vnet）
type connIDer interface {
	ConnID() int64
}

// streamStater 对应 flex 流连接的结构化接口，取 State.Index 作为连接编号
type streamStater interface {
	GetState() *stream.State
}

// closeReader 表示可以半关闭读端的连接（停止接收数据）
type closeReader interface {
	CloseRead() error
}

// closeWriter 表示可以半关闭写端的连接（向对端发送 EOF）
type closeWriter interface {
	CloseWrite() error
}

// networkOfURL 从端点 URL 提取网络别名（即 scheme）：
// vnet 端点形如 alias://vhost:port?authcode=xxx，物理端点保持原始 URL 如 tcp://127.0.0.1:8080。
// 解析失败返回 ""。
func networkOfURL(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return ""
	}
	return u.Scheme
}

// TrackConn 将服务经手的连接登记到 ServiceState，供控制面反查连接归属。
// 能提取到连接编号时返回登记包装（Close 或双向半关闭齐备时注销），
// 提取失败时原样返回 conn（不包装、不影响数据通路）。
func (s *ServiceState) TrackConn(network string, conn net.Conn) net.Conn {
	if network == "" || conn == nil {
		return conn
	}
	index, ok := extractConnIndex(conn)
	if !ok {
		return conn
	}
	ref := connRef{Network: network, Index: index}
	s.connRefs.Store(ref, struct{}{})
	// 底层支持双向半关闭时返回半关闭型包装：utils.RelayConns 通过接口断言
	// CloseRead/CloseWrite 传播单向 EOF，包装类型不能掩盖这两个方法；
	// 底层不支持时则必须返回不带这两个方法的基础型包装。
	rc, okR := conn.(closeReader)
	cw, okW := conn.(closeWriter)
	if okR && okW {
		return &trackedHalfConn{
			trackedConn: trackedConn{Conn: conn, state: s, ref: ref},
			rc:          rc,
			cw:          cw,
		}
	}
	return &trackedConn{Conn: conn, state: s, ref: ref}
}

// MatchConn 查询指定网络内的连接是否由本服务经手（api 层反查用）
func (s *ServiceState) MatchConn(network string, index int64) bool {
	_, ok := s.connRefs.Load(connRef{Network: network, Index: index})
	return ok
}

// untrackConn 注销连接登记
func (s *ServiceState) untrackConn(ref connRef) {
	s.connRefs.Delete(ref)
}

// extractConnIndex 提取连接在网络内的编号。
// 每层依次断言 tcp 追踪接口（ConnID，返回值原样采用）与 flex 流接口
// （GetState，Index 为 0 视为无效）；断言失败时尝试解包加密层
// （见 unwrapCipherConn）后进入下一层重试。
func extractConnIndex(conn net.Conn) (int64, bool) {
	c := conn
	for depth := 0; depth < connUnwrapDepth; depth++ {
		if tc, ok := c.(connIDer); ok {
			return tc.ConnID(), true
		}
		if sc, ok := c.(streamStater); ok {
			if st := sc.GetState(); st != nil && st.Index != 0 {
				return int64(st.Index), true
			}
		}
		inner, ok := unwrapCipherConn(c)
		if !ok {
			return 0, false
		}
		c = inner
	}
	return 0, false
}

// unwrapCipherConn 解包 cipherconn 加密连接，拿到底层连接。
// cipherconn（v1.0.2）未提供导出的解包方法，但其（未导出的）结构体内嵌了
// 导出的 net.Conn 字段，可通过反射读取；该实现对内嵌导出 net.Conn 字段的
// 其他包装结构同样适用。解包仅用于编号断言，数据通路仍走原始连接。
// 对不具备该结构的连接返回 false。
func unwrapCipherConn(conn net.Conn) (net.Conn, bool) {
	v := reflect.ValueOf(conn)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, false
	}
	f := v.FieldByName("Conn")
	if !f.IsValid() || !f.CanInterface() {
		return nil, false
	}
	inner, ok := f.Interface().(net.Conn)
	if !ok || inner == nil {
		return nil, false
	}
	return inner, true
}

// trackedConn 连接登记包装（基础型）：Close 时注销登记并转发关闭。
// 注销幂等且线程安全（utils.RelayConns 的两个 goroutine 会并发操作同一连接）。
type trackedConn struct {
	net.Conn
	state *ServiceState
	ref   connRef
	once  sync.Once
}

func (c *trackedConn) Close() error {
	err := c.Conn.Close()
	c.untrack()
	return err
}

func (c *trackedConn) untrack() {
	c.once.Do(func() { c.state.untrackConn(c.ref) })
}

// trackedHalfConn 连接登记包装（半关闭型）：底层支持 CloseRead/CloseWrite 时使用，
// 保活 utils.RelayConns 依赖的半关闭接口；双向半关闭齐备时同样注销登记。
type trackedHalfConn struct {
	trackedConn
	rc     closeReader
	cw     closeWriter
	halves int32 // 使用 atomic 操作，记录已半关闭的方向数
}

func (c *trackedHalfConn) CloseRead() error {
	err := c.rc.CloseRead()
	if err == nil {
		c.halfClosed()
	}
	return err
}

func (c *trackedHalfConn) CloseWrite() error {
	err := c.cw.CloseWrite()
	if err == nil {
		c.halfClosed()
	}
	return err
}

// halfClosed 双向半关闭齐备（读+写各关闭一次）时注销登记
func (c *trackedHalfConn) halfClosed() {
	if atomic.AddInt32(&c.halves, 1) == 2 {
		c.untrack()
	}
}
