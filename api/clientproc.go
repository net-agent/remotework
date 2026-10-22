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
	// clientProcMaxEntries 解析结果条目上限：高 churn 场景下条目会随流速率增长，
	// 超过上限按插入顺序做简单 FIFO 淘汰，保证内存有界
	clientProcMaxEntries = 4096
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
//
// 加锁约定：entries 锁（r.mu）内绝不做系统调用。端点表枚举（listEndpoints）
// 通过 refreshing 单飞标志在锁外执行，避免慢速 netstat 阻塞所有快照/事件。
type clientProcResolver struct {
	mu      sync.Mutex
	entries map[streamIdentity]*clientProcEntry
	// order 记录条目插入顺序，供容量上限的 FIFO 淘汰；orderHead 之前的槽位已失效
	order     []streamIdentity
	orderHead int

	table      map[utils.TCPEndpointKey]int32
	tableAt    time.Time
	refreshing bool // 端点表枚举单飞标志：同一时刻只允许一个 goroutine 执行系统调用

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
	id := streamIdentity{
		network:   network,
		index:     st.Index,
		createdNs: st.Created.UnixNano(),
		local:     st.LocalAddr.String(),
		remote:    st.RemoteAddr.String(),
	}

	r.mu.Lock()
	e, exists := r.entries[id]
	if !exists {
		e = &clientProcEntry{firstAt: now, endpoint: endpoint, hasEndpoint: true}
		r.addEntryLocked(id, e)
	}
	e.touchedAt = now
	if e.resolved {
		// 命中记忆：单次加锁完成 DTO 填充，不触发任何系统调用
		dto.ClientPid = e.pid
		dto.ClientProcName = e.name
		dto.ClientProcStartMs = e.startMs
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	// 系统调用只在锁外执行
	r.ensureTable(now)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.tryResolveLocked(e, now)
	dto.ClientPid = e.pid
	dto.ClientProcName = e.name
	dto.ClientProcStartMs = e.startMs
}

// tryResolveLocked 查表推进单条条目的解析：查到端点归属即终态
// （pid<=0 视为合法的"未知"，不再重试）；窗口届满仍未命中放弃解析，字段留空。
// 仅读取 r.table，不执行系统调用。
func (r *clientProcResolver) tryResolveLocked(e *clientProcEntry, now time.Time) {
	if r.table != nil {
		if pid, found := r.table[e.endpoint]; found {
			e.resolved = true
			if pid > 0 {
				e.pid = pid
				e.name = r.procName(pid)
				e.startMs = r.procStartMs(pid)
			}
			return
		}
	}
	if now.Sub(e.firstAt) >= clientProcResolveWindow {
		e.resolved = true // 重试窗口届满，放弃解析
	}
}

// ensureTable 在缓存过期时重建端点表；refreshing 保证同一时刻只有一个
// goroutine 在锁外执行全量枚举，其余调用立即返回并复用旧表（下一 tick 再补查）。
func (r *clientProcResolver) ensureTable(now time.Time) {
	r.mu.Lock()
	if r.refreshing || now.Sub(r.tableAt) < clientProcTableTTL {
		r.mu.Unlock()
		return
	}
	r.refreshing = true
	r.mu.Unlock()

	table, err := r.listEndpoints()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		r.table = table
	}
	// 枚举失败也推进 tableAt：避免系统调用失败期间每条流都触发全量枚举
	r.tableAt = now
	r.refreshing = false
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
	pending := false
	for _, e := range r.entries {
		if !e.resolved && e.hasEndpoint {
			pending = true
			break
		}
	}
	r.mu.Unlock()
	if !pending {
		return
	}

	r.ensureTable(now)

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.resolved || !e.hasEndpoint {
			continue
		}
		r.tryResolveLocked(e, now)
	}
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
	r.compactOrderLocked()
}

// addEntryLocked 登记新条目并维持容量上限：超限时按插入顺序淘汰最旧条目。
// 调用方必须持有 r.mu。
func (r *clientProcResolver) addEntryLocked(id streamIdentity, e *clientProcEntry) {
	r.entries[id] = e
	r.order = append(r.order, id)
	for len(r.entries) > clientProcMaxEntries && r.orderHead < len(r.order) {
		evict := r.order[r.orderHead]
		r.orderHead++
		delete(r.entries, evict)
	}
	r.compactOrderLocked()
}

// compactOrderLocked 回收 order 前缀，避免无界增长。调用方必须持有 r.mu。
func (r *clientProcResolver) compactOrderLocked() {
	if r.orderHead == 0 {
		return
	}
	if r.orderHead >= 1024 && r.orderHead*2 >= len(r.order) {
		r.order = append([]streamIdentity(nil), r.order[r.orderHead:]...)
		r.orderHead = 0
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
