package vnet

import (
	"sync"

	"github.com/net-agent/flex/v3/stream"
)

// maxClosedStreamHist 是已关闭流历史的保留上限，对齐 flex 侧环形缓冲容量（1024）
const maxClosedStreamHist = 1024

// streamStateSource 抽象 *node.Node 的流状态读取能力，便于单测 fake
type streamStateSource interface {
	GetStreamStates() []*stream.State
	GetClosedStates(pos int64) ([]*stream.State, int64)
}

// streamStateTracker 把 flex 的增量游标语义聚合成 (actives, closeds) 快照。
// 由调用方驱动（拉模式），无后台 goroutine。
type streamStateTracker struct {
	mu     sync.Mutex
	source streamStateSource // 上次见到的来源实例；Session 重连后会换新 Node
	pos    int64             // GetClosedStates 增量游标
	hist   stateRing         // 已关闭流历史，有界（固定环形缓冲，避免每次淘汰复制整个切片）
}

// collect 采集一次快照。src 为 nil 表示 session 离线/重连中：actives 为空，仅返回历史。
func (t *streamStateTracker) collect(src streamStateSource) (actives, closeds []*stream.State) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if src == nil {
		return nil, t.hist.snapshot()
	}

	// 重连换了新 Node：新实例的环形缓冲从 0 开始，游标重置；
	// hist 保留，旧流的历史记录对报表仍然有效
	if src != t.source {
		t.source = src
		t.pos = 0
	}

	batch, next := src.GetClosedStates(t.pos)
	t.pos = next
	for _, st := range batch {
		t.hist.push(st, maxClosedStreamHist)
	}

	return src.GetStreamStates(), t.hist.snapshot()
}

// stateRing 是固定容量的环形缓冲：push 只覆盖最旧槽位，避免历史满后
// 每次关闭都复制整个切片；snapshot 按写入先后顺序返回快照。
type stateRing struct {
	buf  []*stream.State
	next int // 下一个写入槽位
	size int // 当前元素数（<= len(buf)）
}

func (r *stateRing) push(st *stream.State, capacity int) {
	if capacity <= 0 {
		return
	}
	if len(r.buf) != capacity {
		// 容量固定；仅在首次使用（或容量变化）时分配
		r.buf = make([]*stream.State, capacity)
		r.next = 0
		r.size = 0
	}
	if r.size < capacity {
		r.size++
	}
	r.buf[r.next] = st
	r.next++
	if r.next == capacity {
		r.next = 0
	}
}

// snapshot 返回按写入顺序（旧→新）排列的切片；调用方可以安全修改切片本身。
func (r *stateRing) snapshot() []*stream.State {
	if r.size == 0 || len(r.buf) == 0 {
		return nil
	}
	start := r.next - r.size
	if start < 0 {
		start += len(r.buf)
	}
	out := make([]*stream.State, 0, r.size)
	for i := 0; i < r.size; i++ {
		out = append(out, r.buf[(start+i)%len(r.buf)])
	}
	return out
}
