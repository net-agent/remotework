package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Event 推送给客户端的事件信封
type Event struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// subscribeMsg 客户端发送的订阅消息
type subscribeMsg struct {
	Action string   `json:"action"`
	Events []string `json:"events"`
}

type WSHub struct {
	mu      sync.RWMutex
	clients map[*WSClient]struct{}
	log     *slog.Logger
}

type WSClient struct {
	conn *websocket.Conn
	send chan []byte
	mu   sync.RWMutex
	subs map[string]bool // 订阅的事件类型，nil 表示全部
}

func NewWSHub(log *slog.Logger) *WSHub {
	return &WSHub{
		clients: make(map[*WSClient]struct{}),
		log:     log,
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
	close(c.send)
}

// Broadcast 向所有订阅了该事件类型的客户端广播事件
func (h *WSHub) Broadcast(eventType string, data interface{}) {
	evt := Event{
		Type:      eventType,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}
	buf, err := json.Marshal(evt)
	if err != nil {
		h.log.Error("marshal event failed", "err", err)
		return
	}

	h.mu.RLock()
	snapshot := make([]*WSClient, 0, len(h.clients))
	for c := range h.clients {
		snapshot = append(snapshot, c)
	}
	h.mu.RUnlock()

	for _, c := range snapshot {
		if c.isSubscribed(eventType) {
			select {
			case c.send <- buf:
			default:
				// 客户端发送缓冲区满，跳过
			}
		}
	}
}

// CloseAll 关闭所有客户端连接
func (h *WSHub) CloseAll() {
	h.mu.Lock()
	clients := make([]*WSClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		c.conn.Close()
	}
}

func (c *WSClient) isSubscribed(eventType string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.subs == nil {
		return true // nil 表示订阅全部
	}
	return c.subs[eventType]
}

func (c *WSClient) setSubscriptions(events []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = make(map[string]bool, len(events))
	for _, e := range events {
		c.subs[e] = true
	}
}

// handleWebSocket 处理 WebSocket 连接升级
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("websocket upgrade failed", "err", err)
		return
	}

	client := &WSClient{
		conn: conn,
		send: make(chan []byte, 64),
	}
	s.wsHub.addClient(client)

	go s.wsWritePump(client)
	go s.wsReadPump(client)
}

// wsReadPump 读取客户端消息（订阅指令），5秒内未订阅则自动订阅全部
func (s *Server) wsReadPump(c *WSClient) {
	defer func() {
		s.wsHub.removeClient(c)
		c.conn.Close()
	}()

	// 5秒内未收到订阅消息则自动订阅全部
	subscribed := make(chan struct{}, 1)
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-subscribed:
		case <-timer.C:
			// 超时，subs 保持 nil 即订阅全部
		}
	}()

	c.conn.SetReadDeadline(time.Time{})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var sub subscribeMsg
		if json.Unmarshal(msg, &sub) == nil && sub.Action == "subscribe" {
			c.setSubscriptions(sub.Events)
			select {
			case subscribed <- struct{}{}:
			default:
			}
		}
	}
}

// wsWritePump 将事件写入 WebSocket 连接
func (s *Server) wsWritePump(c *WSClient) {
	defer c.conn.Close()

	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
