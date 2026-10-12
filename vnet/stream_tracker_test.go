package vnet

import (
	"fmt"
	"testing"

	"github.com/net-agent/flex/v3/stream"
)

// fakeStreamSource 模拟 flex Node 的流状态读取：closed 为全量环形缓冲，按 pos 增量返回
type fakeStreamSource struct {
	actives []*stream.State
	closed  []*stream.State
}

func (f *fakeStreamSource) GetStreamStates() []*stream.State { return f.actives }

func (f *fakeStreamSource) GetClosedStates(pos int64) ([]*stream.State, int64) {
	seq := int64(len(f.closed))
	if pos >= seq {
		return nil, seq
	}
	return f.closed[pos:], seq
}

func makeStates(prefix string, n int) []*stream.State {
	states := make([]*stream.State, 0, n)
	for i := 0; i < n; i++ {
		states = append(states, &stream.State{LocalDomain: fmt.Sprintf("%s-%d", prefix, i)})
	}
	return states
}

func TestStreamTracker_FirstCollect(t *testing.T) {
	src := &fakeStreamSource{actives: makeStates("active", 2), closed: makeStates("closed", 3)}
	var tracker streamStateTracker

	actives, closeds := tracker.collect(src)
	if len(actives) != 2 {
		t.Errorf("actives = %d, want 2", len(actives))
	}
	if len(closeds) != 3 {
		t.Errorf("closeds = %d, want 3", len(closeds))
	}
}

func TestStreamTracker_IncrementalNoDuplicate(t *testing.T) {
	src := &fakeStreamSource{closed: makeStates("closed", 3)}
	var tracker streamStateTracker

	tracker.collect(src)
	src.closed = append(src.closed, makeStates("closed", 2)...) // 新关闭 2 条

	_, closeds := tracker.collect(src)
	if len(closeds) != 5 {
		t.Fatalf("closeds = %d, want 5", len(closeds))
	}

	// 没有新增时再次 collect，历史不应重复增长
	_, closeds = tracker.collect(src)
	if len(closeds) != 5 {
		t.Errorf("closeds = %d, want 5 (no duplicate)", len(closeds))
	}
}

func TestStreamTracker_SourceSwitchResetsCursor(t *testing.T) {
	var tracker streamStateTracker

	old := &fakeStreamSource{closed: makeStates("old", 3)}
	tracker.collect(old)

	// 重连后 Session 换成新 Node，其环形缓冲从 0 开始
	fresh := &fakeStreamSource{closed: makeStates("new", 2)}
	_, closeds := tracker.collect(fresh)

	// 旧历史保留 + 新 Node 全量 2 条
	if len(closeds) != 5 {
		t.Fatalf("closeds = %d, want 5", len(closeds))
	}
	if closeds[3].LocalDomain != "new-0" {
		t.Errorf("closeds[3].LocalDomain = %q, want %q", closeds[3].LocalDomain, "new-0")
	}

	// 新 source 上游标正常推进
	fresh.closed = append(fresh.closed, makeStates("new", 1)...)
	_, closeds = tracker.collect(fresh)
	if len(closeds) != 6 {
		t.Errorf("closeds = %d, want 6", len(closeds))
	}
}

func TestStreamTracker_HistoryBounded(t *testing.T) {
	src := &fakeStreamSource{closed: makeStates("closed", maxClosedStreamHist)}
	var tracker streamStateTracker

	tracker.collect(src)
	src.closed = append(src.closed, makeStates("closed", 10)...)

	_, closeds := tracker.collect(src)
	if len(closeds) != maxClosedStreamHist {
		t.Fatalf("closeds = %d, want %d", len(closeds), maxClosedStreamHist)
	}
	// 最旧的 10 条被淘汰
	if closeds[0].LocalDomain != "closed-10" {
		t.Errorf("closeds[0].LocalDomain = %q, want %q", closeds[0].LocalDomain, "closed-10")
	}
}

func TestStreamTracker_NilSourceReturnsHistory(t *testing.T) {
	src := &fakeStreamSource{closed: makeStates("closed", 2)}
	var tracker streamStateTracker
	tracker.collect(src)

	actives, closeds := tracker.collect(nil)
	if actives != nil {
		t.Errorf("actives = %v, want nil", actives)
	}
	if len(closeds) != 2 {
		t.Errorf("closeds = %d, want 2", len(closeds))
	}
}

func TestStreamTracker_ReturnsCopy(t *testing.T) {
	src := &fakeStreamSource{closed: makeStates("closed", 1)}
	var tracker streamStateTracker

	_, closeds := tracker.collect(src)
	closeds[0] = nil // 修改返回值不应影响内部历史

	_, closeds = tracker.collect(src)
	if closeds[0] == nil {
		t.Error("internal history was mutated through returned slice")
	}
}
