package api

import (
	"sync"
	"testing"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

func TestStreamMeterFirstSampleZero(t *testing.T) {
	m := newStreamMeter()
	st := &stream.State{Index: 1, BytesRead: 1000, BytesWritten: 500}

	rates := m.sample("mynet", []*stream.State{st}, time.Now())
	rate, ok := rates[1]
	if !ok {
		t.Fatal("rate for index 1 missing")
	}
	// 首次采样无上次快照，速率为 0
	if rate[0] != 0 || rate[1] != 0 {
		t.Fatalf("first sample: want [0 0], got %v", rate)
	}
}

func TestStreamMeterRateCompute(t *testing.T) {
	m := newStreamMeter()
	t0 := time.Now()
	st := &stream.State{Index: 1, BytesRead: 1000, BytesWritten: 500}

	m.sample("mynet", []*stream.State{st}, t0)

	// 2 秒后读 +2000 写 +1000 → 读 1000 B/s、写 500 B/s
	st.BytesRead = 3000
	st.BytesWritten = 1500
	rates := m.sample("mynet", []*stream.State{st}, t0.Add(2*time.Second))
	rate := rates[1]
	if rate[0] != 1000 || rate[1] != 500 {
		t.Fatalf("rate: want [1000 500], got %v", rate)
	}

	// 字节数不变 → 速率为 0
	rates = m.sample("mynet", []*stream.State{st}, t0.Add(4*time.Second))
	if r := rates[1]; r[0] != 0 || r[1] != 0 {
		t.Fatalf("idle rate: want [0 0], got %v", r)
	}
}

func TestStreamMeterCounterReset(t *testing.T) {
	m := newStreamMeter()
	t0 := time.Now()
	st := &stream.State{Index: 1, BytesRead: 5000, BytesWritten: 5000}
	m.sample("mynet", []*stream.State{st}, t0)

	// 计数器回退（网络重连重置）时速率归零而非负值
	st.BytesRead = 100
	st.BytesWritten = 100
	rates := m.sample("mynet", []*stream.State{st}, t0.Add(time.Second))
	if r := rates[1]; r[0] != 0 || r[1] != 0 {
		t.Fatalf("counter reset: want [0 0], got %v", r)
	}
}

func TestStreamMeterEviction(t *testing.T) {
	m := newStreamMeter()
	t0 := time.Now()

	m.sample("mynet", []*stream.State{{Index: 1}}, t0)
	m.sample("other", []*stream.State{{Index: 9}}, t0)

	// 第 1 轮缺席：未淘汰
	m.sample("mynet", nil, t0.Add(time.Second))
	if got := len(m.entries); got != 2 {
		t.Fatalf("after 1 miss: want 2 entries, got %d", got)
	}

	// 第 2 轮缺席：淘汰（仅淘汰 mynet 的键，other 不受影响）
	m.sample("mynet", nil, t0.Add(2*time.Second))
	if got := len(m.entries); got != 1 {
		t.Fatalf("after 2 misses: want 1 entry, got %d", got)
	}
	if _, ok := m.entries[meterKey{network: "other", index: 9}]; !ok {
		t.Fatal("other network entry should not be evicted")
	}

	// 被淘汰后重新出现：视为首次采样，速率为 0
	rates := m.sample("mynet", []*stream.State{{Index: 1, BytesRead: 100}}, t0.Add(3*time.Second))
	if r := rates[1]; r[0] != 0 || r[1] != 0 {
		t.Fatalf("re-appeared stream: want [0 0], got %v", r)
	}
}

func TestStreamMeterConcurrent(t *testing.T) {
	m := newStreamMeter()
	st := &stream.State{Index: 1, BytesRead: 1000, BytesWritten: 500}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				m.sample("mynet", []*stream.State{st}, time.Now())
			}
		}(i)
	}
	wg.Wait()
}
