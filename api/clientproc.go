package api

import (
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/utils"
)

// 客户端进程归属解析的缓存参数
const (
	// clientProcTableTTL 本机 TCP 端点表缓存时长：有效期内流打开直接查表，
	// 过期才重新枚举（枚举是全量系统调用，不能在每条新流上无缓存触发）
	clientProcTableTTL = time.Second
	// clientProcResolveWindow 单条流的解析重试窗口：首次查表可能因缓存表早于
	// 连接建立而 miss，窗口内后续快照/事件会重试；窗口届满放弃，字段留空（不视为错误）
	clientProcResolveWindow = 10 * time.Second
	// clientProcEntryIdleTTL 单条流解析结果的闲置保留时长：活跃流每次被引用都会刷新，
	// 流关闭后结果仍保留一段窗口供 stream.closed / REST 历史透传
	clientProcEntryIdleTTL = 10 * time.Minute
)

// streamIdentity 一条流的稳定标识：四元组 + 序号 + 创建时刻。
// 端口复用或会话重连不会命中旧流的缓存（createdNs 必然不同）
type streamIdentity struct {
	network   string
	index     int32
	createdNs int64
	local     string
	remote    string
}

// clientProcEntry 单条流的解析结果；resolved=false 表示尚未查到，允许重试
type clientProcEntry struct {
	pid       int32
	name      string
	startMs   int64
	resolved  bool
	firstAt   time.Time // 首次解析尝试时刻，用于界定重试窗口
	touchedAt time.Time // 最近一次被快照/事件引用，用于闲置淘汰

	// endpoint 记录待解析的端点：已关闭的短连接不再出现在 stats 通道，
	// 但重试窗口内（TIME_WAIT 残留仍带属主）可由 retryPending 补查
	endpoint    utils.TCPEndpointKey
	hasEndpoint bool
}

// clientProcResolver 把「inbound 且 remoteAddr 为 loopback」的流反查为本机客户端进程。
// 解析结果按流记忆一次，后续快照与事件透传；线程安全。
type clientProcResolver struct {
	mu      sync.Mutex
	entries map[streamIdentity]*clientProcEntry
	table   map[utils.TCPEndpointKey]int32
	tableAt time.Time

	// 以下依赖做成可替换字段，便于单测（默认走 gopsutil 真实系统调用）
	now           func() time.Time
	listEndpoints func() (map[utils.TCPEndpointKey]int32, error)
	procName      func(pid int32) string
	procStartMs   func(pid int32) int64
}

func newClientProcResolver() *clientProcResolver {
	return &clientProcResolver{
		entries:       make(map[streamIdentity]*clientProcEntry),
		now:           time.Now,
		listEndpoints: utils.ListTCPLocalEndpoints,
		procName:      utils.GetProcessName,
		procStartMs:   utils.GetProcessStartMs,
	}
}

// enrich 把客户端进程信息填充到 DTO（仅 inbound 且 remoteAddr 为 loopback 的流）；
// 其他流与解析失败场景字段保持零值（omitempty 省略），不视为错误。nil 接收者安全。
func (r *clientProcResolver) enrich(network string, st *stream.State, dto *StreamStateDTO) {
	if r == nil || st == nil || dto == nil {
		return
	}
	if st.Direction != stream.DirectionInbound {
		return
	}
	endpoint, ok := loopbackEndpointOf(st)
	if !ok {
		return
	}

	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()

	id := streamIdentity{
		network:   network,
		index:     st.Index,
		createdNs: st.Created.UnixNano(),
		local:     st.LocalAddr.String(),
		remote:    st.RemoteAddr.String(),
	}
	e, exists := r.entries[id]
	if !exists {
		e = &clientProcEntry{firstAt: now, endpoint: endpoint, hasEndpoint: true}
		r.entries[id] = e
	}
	e.touchedAt = now

	if !e.resolved {
		r.tryResolveLocked(e, now)
	}

	dto.ClientPid = e.pid
	dto.ClientProcName = e.name
	dto.ClientProcStartMs = e.startMs
}

// tryResolveLocked 查表推进单条目的解析：查到端点归属即终态
// （pid<=0 视为合法的"未知"，不再重试）；窗口届满仍未命中放弃解析，字段留空
func (r *clientProcResolver) tryResolveLocked(e *clientProcEntry, now time.Time) {
	if pid, found := r.lookupLocked(e.endpoint, now); found {
		e.resolved = true
		if pid > 0 {
			e.pid = pid
			e.name = r.procName(pid)
			e.startMs = r.procStartMs(pid)
		}
	} else if now.Sub(e.firstAt) >= clientProcResolveWindow {
		e.resolved = true // 重试窗口届满，放弃解析
	}
}

// retryPending 对仍处于重试窗口内的未解析条目补查一轮，由 stats tick 驱动
// （pollStreamStats）。已关闭的短连接不再出现在 stats 通道，若仅在 enrich 时
// 重试会形成「开流 miss → 秒关 → 永不重试」；TIME_WAIT 残留在窗口期内仍保留
// 属主，此类连接由此可补解析（结果供 REST /streams 历史透传）。nil 接收者安全。
func (r *clientProcResolver) retryPending() {
	if r == nil {
		return
	}
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.resolved || !e.hasEndpoint {
			continue
		}
		r.tryResolveLocked(e, now)
	}
}

// lookupLocked 查端点表，缓存过期时先重建；枚举失败沿用旧表并把 tableAt 照常推进，
// 避免系统调用失败期间每条流都触发全量枚举
func (r *clientProcResolver) lookupLocked(endpoint utils.TCPEndpointKey, now time.Time) (int32, bool) {
	if now.Sub(r.tableAt) >= clientProcTableTTL {
		if table, err := r.listEndpoints(); err == nil {
			r.table = table
		}
		r.tableAt = now
	}
	pid, found := r.table[endpoint]
	return pid, found
}

// sweep 淘汰闲置超过 clientProcEntryIdleTTL 的解析结果；nil 接收者安全
func (r *clientProcResolver) sweep(now time.Time) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.entries {
		if now.Sub(e.touchedAt) >= clientProcEntryIdleTTL {
			delete(r.entries, id)
		}
	}
}

// loopbackEndpointOf 从流状态提取 loopback 端点键；非 loopback 或地址无法解析时 ok=false
func loopbackEndpointOf(st *stream.State) (utils.TCPEndpointKey, bool) {
	var zero utils.TCPEndpointKey
	host, portText, err := net.SplitHostPort(st.RemoteAddr.String())
	if err != nil {
		return zero, false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return zero, false
	}
	if ip = ip.Unmap(); !ip.IsLoopback() {
		return zero, false
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return zero, false
	}
	return utils.TCPEndpointKey{IP: ip.String(), Port: uint16(port)}, true
}
