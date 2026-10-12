package kernel

import (
	"sync"
	"time"
)

//
// 内核预置能力：事件总线与插件目录。
// 宿主在 NewHost 时以属主 "kernel" 发布，任何插件可通过
// Inject[EventBus](c, "") / Inject[Catalog](c, "") 获取。
//

// DomainEvent 领域事件：插件经事件总线交换的运行时消息（如网络会话状态变化）。
// 与插件生命周期 Event 不同，DomainEvent 不经过调度器，由发布者直接扇出。
type DomainEvent struct {
	Time    time.Time
	Topic   string
	Payload any
}

// EventBus 领域事件总线。Publish 可从任意 goroutine 调用，非阻塞：
// 订阅者 channel 满时丢弃该条事件（订阅方应给足缓冲）。
type EventBus interface {
	Publish(topic string, payload any)
	// SubscribeTopic 订阅指定主题；返回的函数用于退订
	SubscribeTopic(buf int, topics ...string) (<-chan DomainEvent, func())
}

// Catalog 插件目录与生命周期管理，由宿主实现并预置。
// 观察器类插件通过它查询状态、订阅生命周期事件、动态装载/卸载插件。
type Catalog interface {
	List() []PluginState
	State(name string) (PluginState, error)
	Subscribe(buf int) (<-chan Event, func())
	Load(p Plugin) error
	Unload(name string, opts UnloadOptions) error
	Retry(name string) error
}

var (
	_ EventBus = (*eventBus)(nil)
	_ Catalog  = (*Host)(nil)
)

type eventBus struct {
	mu     sync.RWMutex
	subs   map[chan DomainEvent]map[string]bool
	closed bool
}

func newEventBus() *eventBus {
	return &eventBus{subs: make(map[chan DomainEvent]map[string]bool)}
}

func (b *eventBus) Publish(topic string, payload any) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	evt := DomainEvent{Time: time.Now(), Topic: topic, Payload: payload}
	for ch, topics := range b.subs {
		if !topics[topic] {
			continue
		}
		select {
		case ch <- evt:
		default:
		}
	}
}

func (b *eventBus) SubscribeTopic(buf int, topics ...string) (<-chan DomainEvent, func()) {
	if buf <= 0 {
		buf = 64
	}
	ch := make(chan DomainEvent, buf)
	set := make(map[string]bool, len(topics))
	for _, t := range topics {
		set[t] = true
	}
	b.mu.Lock()
	if b.closed {
		close(ch)
		b.mu.Unlock()
		return ch, func() {}
	}
	b.subs[ch] = set
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.closed {
				return // close() 已关闭所有订阅 channel
			}
			delete(b.subs, ch)
			close(ch)
		})
	}
}

func (b *eventBus) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}
