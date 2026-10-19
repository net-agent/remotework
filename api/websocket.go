package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WS 心跳参数：服务端按 pingPeriod 发 ping，客户端超过 pongWait 未回 pong 即判定死连。
// writePump 仅在写消息时占用连接，写超时用 writeWait 兜底。
const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 30 * time.Second // 必须小于 wsPongWait
)

// 客户端发送缓冲区容量；溢出策略见 Broadcast 的 droppable 语义
const wsSendBufferSize = 256

var upgrader = websocket.Upgrader{
	// Origin 为空（非浏览器客户端）直接允许；否则必须在白名单内
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || allowedOrigins[origin]
	},
}

// Event 推送给客户端的事件信封。
// Topic/Seq 为 sidecar 协议 v2 新增（additive）：
// Topic 为事件主题（事件类型 "." 前缀），Seq 在主题内单调递增；
// 快照事件（*.snapshot）的 Seq 表示"本快照已覆盖 ≤ Seq 的全部增量事件"。
type Event struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Topic     string      `json:"topic"`
	Seq       int64       `json:"seq"`
	Data      interface{} `json:"data"`
}

// topicOf 从事件类型提取主题（"." 前缀）
func topicOf(eventType string) string {
	if i := strings.IndexByte(eventType, '.'); i > 0 {
		return eventType[:i]
	}
	return eventType
}

// subscribeMsg 客户端发送的订阅消息。
// Topics 与 Events 互斥（同一条消息同时携带时 Topics 优先）：
//   - Topics：v2 主题订阅，按主题过滤并对每个主题回发 *.snapshot；空数组 = 退订全部
//   - Events：v1 事件类型过滤（保持兼容，不触发快照）；空数组 = 退订全部
//   - 两者都缺省：不改变现有订阅
type subscribeMsg struct {
	Action string   `json:"action"`
	Events []string `json:"events"`
	Topics []string `json:"topics"`
}

// resyncMsg 客户端请求主题级重同步（seq 空洞或本地状态异常时）；
// 服务端对每个主题重发快照（无论是否已订阅该主题）
type resyncMsg struct {
	Action string   `json:"action"`
	Topics []string `json:"topics"`
}

// snapshotter 生成指定主题的快照事件；返回 ok=false 表示主题不存在。
// 实现在 Server 侧（snapshot.go），由 readPump 在处理订阅/重同步时回调。
type snapshotter func(topic string) (eventType string, data interface{}, ok bool)

type WSHub struct {
	mu      sync.RWMutex
	clients map[*WSClient]struct{}
	log     *slog.Logger

	topicsMu sync.Mutex
	seqs     map[string]int64 // 主题 -> 已分配的最大序列号
}

type WSClient struct {
	conn   *websocket.Conn
	send   chan []byte
	kicked chan struct{} // 可靠事件投递失败时关闭，通知 writePump 断连

	mu     sync.RWMutex
	events map[string]bool // 事件类型过滤（v1 events 订阅），nil 表示不过滤
	topics map[string]bool // 主题过滤（v2 topics 订阅），nil 表示不过滤

	snapshotFn snapshotter // 由 Server 注入；nil 时 resync/topics 快照不生效
}

func NewWSHub(log *slog.Logger) *WSHub {
	return &WSHub{
		clients: make(map[*WSClient]struct{}),
		log:     log,
		seqs:    make(map[string]int64),
	}
}

func (h *WSHub) addClient(c *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *WSHub) removeClient(c *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	// 扇出全程持读锁（broadcast/sendSnapshot），此处写锁与其互斥，
	// 关闭 send 不会与推送交错
	close(c.send)
}

// Broadcast 广播可靠事件（生命周期/状态类）：客户端缓冲区满时踢掉该客户端
// （断连重连后走快照+增量恢复），保证事件不静默丢失。
func (h *WSHub) Broadcast(eventType string, data interface{}) {
	h.broadcast(eventType, data, false)
}

// BroadcastDroppable 广播可丢弃事件（计数器/速率类 stats）：
// 客户端缓冲区满时丢弃该条，客户端可等下一 tick 的全量 stats 补齐。
func (h *WSHub) BroadcastDroppable(eventType string, data interface{}) {
	h.broadcast(eventType, data, true)
}

func (h *WSHub) broadcast(eventType string, data interface{}, droppable bool) {
	topic := topicOf(eventType)
	evt := Event{
		Type:      eventType,
		Timestamp: time.Now().UnixMilli(),
		Topic:     topic,
		Seq:       h.nextSeq(topic),
		Data:      data,
	}
	buf, err := json.Marshal(evt)
	if err != nil {
		h.log.Error("marshal event failed", "err", err)
		return
	}

	// 整个扇出持读锁，保证 removeClient 与推送不会交错
	// （推送均为非阻塞操作，不会长时间持锁）
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.isSubscribed(eventType, topic) {
			continue
		}
		select {
		case c.send <- buf:
		default:
			if droppable {
				continue // stats 类允许丢旧补新
			}
			h.kickLocked(c)
		}
	}
}

// kickLocked 踢掉慢客户端：关闭 kicked 通知 writePump 主动断开连接。
// 调用方必须持有 h.mu（读锁即可，kicked 只关闭一次）。
// 测试路径构造的 WSClient 可能没有 kicked channel，直接跳过。
func (h *WSHub) kickLocked(c *WSClient) {
	if c.kicked == nil {
		return
	}
	select {
	case <-c.kicked:
	default:
		close(c.kicked)
		h.log.Warn("ws client kicked: send buffer full on reliable event")
	}
}

// nextSeq 分配主题内单调递增的序列号
func (h *WSHub) nextSeq(topic string) int64 {
	h.topicsMu.Lock()
	defer h.topicsMu.Unlock()
	h.seqs[topic]++
	return h.seqs[topic]
}

// sendSnapshot 向单个客户端发送主题快照。
// 先登记订阅（readPump 内）再发快照，且 Seq 必须在采集【前】读取：
// 采集期间广播的增量事件 Seq 一定大于快照 Seq，客户端据此保留（而非丢弃）这些事件，
// 配合幂等应用即可收敛；快照事件的 Seq 表示"本快照已覆盖 ≤ Seq 的全部增量事件"。
func (h *WSHub) sendSnapshot(c *WSClient, topic string) {
	if c.snapshotFn == nil {
		return
	}

	h.topicsMu.Lock()
	seq := h.seqs[topic]
	h.topicsMu.Unlock()

	eventType, data, ok := c.snapshotFn(topic)
	if !ok {
		return
	}

	evt := Event{
		Type:      eventType,
		Timestamp: time.Now().UnixMilli(),
		Topic:     topic,
		Seq:       seq,
		Data:      data,
	}
	buf, err := json.Marshal(evt)
	if err != nil {
		h.log.Error("marshal snapshot failed", "topic", topic, "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	select {
	case c.send <- buf:
	default:
		// 快照属可靠投递：发不出去说明客户端已跟不上，踢掉让其重连重来
		h.kickLocked(c)
	}
}

// CloseAll 关闭所有客户端连接
func (h *WSHub) CloseAll() {
	h.mu.RLock()
	clients := make([]*WSClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.conn.Close()
	}
}

func (c *WSClient) isSubscribed(eventType, topic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// topics 过滤优先（v2）；events 过滤兜底（v1）；都为 nil 表示接收全部
	if c.topics != nil {
		return c.topics[topic]
	}
	if c.events != nil {
		return c.events[eventType]
	}
	return true
}

// setTopics 全量替换主题订阅集（v2）；设置后 events 过滤失效。
// 返回本次需要回发快照的主题（新增的订阅项）。
func (c *WSClient) setTopics(topics []string) []string {
	set := make(map[string]bool, len(topics))
	for _, t := range topics {
		set[t] = true
	}
	c.mu.Lock()
	old := c.topics
	c.topics = set
	c.events = nil
	c.mu.Unlock()

	// 旧订阅为 nil（接收全部）时所有主题都视作"新增"，需要快照以建立基线
	var added []string
	for t := range set {
		if old == nil || !old[t] {
			added = append(added, t)
		}
	}
	return added
}

// setEvents 全量替换事件类型订阅集（v1 兼容路径）；设置后 topics 过滤失效
func (c *WSClient) setEvents(events []string) {
	set := make(map[string]bool, len(events))
	for _, e := range events {
		set[e] = true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = set
	c.topics = nil
}

// handleWebSocket 处理 WebSocket 连接升级
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// webview 无法自定义 WS 请求头，token 经查询参数传递
	if s.token != "" && r.URL.Query().Get("token") != s.token {
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("websocket upgrade failed", "err", err)
		return
	}

	client := &WSClient{
		conn:       conn,
		send:       make(chan []byte, wsSendBufferSize),
		kicked:     make(chan struct{}),
		snapshotFn: s.snapshotOf,
	}
	s.wsHub.addClient(client)

	go s.wsWritePump(client)
	go s.wsReadPump(client)
}

// wsReadPump 读取客户端订阅/重同步指令；未发送任何订阅消息时接收全部事件
func (s *Server) wsReadPump(c *WSClient) {
	defer func() {
		s.wsHub.removeClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var sub subscribeMsg
		if json.Unmarshal(msg, &sub) != nil {
			continue
		}
		switch sub.Action {
		case "subscribe":
			if sub.Topics != nil {
				// 先登记订阅再发快照：快照生成后发生的事件都会经正常广播送达
				for _, topic := range c.setTopics(sub.Topics) {
					s.wsHub.sendSnapshot(c, topic)
				}
			} else if sub.Events != nil {
				c.setEvents(sub.Events)
			}
		case "resync":
			for _, topic := range sub.Topics {
				s.wsHub.sendSnapshot(c, topic)
			}
		}
	}
}

// wsWritePump 将事件写入 WebSocket 连接，并周期发送 ping 保活
func (s *Server) wsWritePump(c *WSClient) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return // removeClient 关闭 send，连接已由 readPump 侧关闭
			}
			c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.kicked:
			// 可靠事件投递失败：主动断连，客户端重连后走快照+增量恢复
			c.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(time.Second))
			return
		}
	}
}
