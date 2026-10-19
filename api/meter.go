package api

import (
	"sync"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

// meterMaxMiss 流键连续缺席采样达到该轮数后被淘汰（防泄漏）
const meterMaxMiss = 2

// meterKey 流速计缓存键：(网络名, 流 Index)
type meterKey struct {
	network string
	index   int32
}

// meterEntry 单条流的上次采样快照
type meterEntry struct {
	read    int64
	written int64
	at      time.Time
	miss    int // 连续未在采样中出现的轮数
}

// streamMeter 流实时速率计：以 (network, index) 为键缓存上次采样，
// 每次采样按 (字节增量)/(时间增量) 计算速率；线程安全。
type streamMeter struct {
	mu      sync.Mutex
	entries map[meterKey]*meterEntry
}

func newStreamMeter() *streamMeter {
	return &streamMeter{entries: make(map[meterKey]*meterEntry)}
}

// sample 对一个网络的活跃流采样，返回 index -> [readBps, writeBps]（字节/秒）。
// 首次出现的流速率记 0；本轮未出现的旧键累计 miss，达到阈值后淘汰。
func (m *streamMeter) sample(network string, states []*stream.State, now time.Time) map[int32][2]float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	rates := make(map[int32][2]float64, len(states))
	seen := make(map[meterKey]bool, len(states))

	for _, st := range states {
		if st == nil {
			continue
		}
		key := meterKey{network: network, index: st.Index}
		seen[key] = true

		var rate [2]float64
		if prev, ok := m.entries[key]; ok {
			dt := now.Sub(prev.at).Seconds()
			if dt > 0 {
				rate[0] = float64(st.BytesRead-prev.read) / dt
				rate[1] = float64(st.BytesWritten-prev.written) / dt
				// 计数器回退（如网络重连重置）时速率归零，避免负值
				if rate[0] < 0 {
					rate[0] = 0
				}
				if rate[1] < 0 {
					rate[1] = 0
				}
			}
			prev.read = st.BytesRead
			prev.written = st.BytesWritten
			prev.at = now
			prev.miss = 0
		} else {
			m.entries[key] = &meterEntry{read: st.BytesRead, written: st.BytesWritten, at: now}
		}
		rates[st.Index] = rate
	}

	// 本网络内本轮缺席的键累计 miss，超阈值淘汰
	for key, e := range m.entries {
		if key.network != network || seen[key] {
			continue
		}
		e.miss++
		if e.miss >= meterMaxMiss {
			delete(m.entries, key)
		}
	}

	return rates
}
