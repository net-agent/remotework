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
	hist   []*stream.State   // 已关闭流历史，有界
}

// collect 采集一次快照。src 为 nil 表示 session 离线/重连中：actives 为空，仅返回历史。
func (t *streamStateTracker) collect(src streamStateSource) (actives, closeds []*stream.State) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if src == nil {
		return nil, append([]*stream.State(nil), t.hist...)
	}

	// 重连换了新 Node：新实例的环形缓冲从 0 开始，游标重置；
	// hist 保留，旧流的历史记录对报表仍然有效
	if src != t.source {
		t.source = src
		t.pos = 0
	}

	batch, next := src.GetClosedStates(t.pos)
	t.pos = next
	t.hist = append(t.hist, batch...)
	if over := len(t.hist) - maxClosedStreamHist; over > 0 {
		t.hist = append([]*stream.State(nil), t.hist[over:]...)
	}

	return src.GetStreamStates(), append([]*stream.State(nil), t.hist...)
}
