package api

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/net-agent/remotework/utils"
)

// TestClientProcEntryCap 解析结果条目上限：高 churn 下内存有界，FIFO 淘汰最旧条目
func TestClientProcEntryCap(t *testing.T) {
	f := newClientProcFixture(nil)

	total := clientProcMaxEntries + 64
	for i := 0; i < total; i++ {
		st := makeInboundLoopbackStream(fmt.Sprintf("127.0.0.1:%d", 10000+i))
		st.Index = int32(i)
		st.Created = time.Unix(0, int64(i))
		var dto StreamStateDTO
		f.r.enrich("mynet", st, &dto)
	}

	if got := len(f.r.entries); got != clientProcMaxEntries {
		t.Fatalf("entries = %d, want cap %d", got, clientProcMaxEntries)
	}
	if got := f.r.orderHead; got != total-clientProcMaxEntries {
		t.Fatalf("orderHead = %d, want %d (FIFO evicted)", got, total-clientProcMaxEntries)
	}

	// 最新条目仍可命中记忆；最旧条目已被淘汰（再次 enrich 会重新建条目）
	newest := makeInboundLoopbackStream(fmt.Sprintf("127.0.0.1:%d", 10000+total-1))
	newest.Index = int32(total - 1)
	newest.Created = time.Unix(0, int64(total-1))
	beforeCalls := f.tableCalls
	var dto StreamStateDTO
	f.r.enrich("mynet", newest, &dto)
	if f.tableCalls != beforeCalls {
		t.Fatalf("newest entry should be memoized, tableCalls %d -> %d", beforeCalls, f.tableCalls)
	}

	oldest := makeInboundLoopbackStream("127.0.0.1:10000")
	oldest.Index = 0
	oldest.Created = time.Unix(0, 0)
	// 推进时钟让端点表缓存过期，重新建条目时会触发一次枚举
	f.advance(2 * time.Second)
	f.r.enrich("mynet", oldest, &dto)
	if len(f.r.entries) > clientProcMaxEntries {
		t.Fatalf("entries exceeded cap after re-add: %d", len(f.r.entries))
	}
}

// TestClientProcListEndpointsOutsideLock 端点枚举必须在锁外执行（P1-11）：
// 枚举阻塞期间 sweep 仍能拿到锁，不会被慢速 netstat 卡住。
func TestClientProcListEndpointsOutsideLock(t *testing.T) {
	f := newClientProcFixture(nil)

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	tableCalls := 0
	f.r.listEndpoints = func() (map[utils.TCPEndpointKey]int32, error) {
		mu.Lock()
		tableCalls++
		mu.Unlock()
		once.Do(func() { close(started) })
		<-release
		return map[utils.TCPEndpointKey]int32{{IP: "127.0.0.1", Port: 10001}: 100}, nil
	}

	st := makeInboundLoopbackStream("127.0.0.1:10001")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var dto StreamStateDTO
		f.r.enrich("mynet", st, &dto)
	}()

	<-started

	sweepDone := make(chan struct{})
	go func() {
		f.r.sweep(f.now)
		close(sweepDone)
	}()
	select {
	case <-sweepDone:
	case <-time.After(time.Second):
		t.Fatal("resolver mutex held while listEndpoints is running")
	}

	close(release)
	wg.Wait()

	mu.Lock()
	calls := tableCalls
	mu.Unlock()
	if calls != 1 {
		t.Fatalf("tableCalls = %d, want 1", calls)
	}
	if len(f.r.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(f.r.entries))
	}
}

// TestClientProcSingleFlightRefresh 并发未解析流只触发一次端点枚举（单飞标志）
func TestClientProcSingleFlightRefresh(t *testing.T) {
	f := newClientProcFixture(nil)

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	tableCalls := 0
	f.r.listEndpoints = func() (map[utils.TCPEndpointKey]int32, error) {
		mu.Lock()
		tableCalls++
		mu.Unlock()
		once.Do(func() { close(started) })
		<-release
		return map[utils.TCPEndpointKey]int32{}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		st := makeInboundLoopbackStream(fmt.Sprintf("127.0.0.1:%d", 20000+i))
		st.Index = int32(i)
		go func() {
			defer wg.Done()
			var dto StreamStateDTO
			f.r.enrich("mynet", st, &dto)
		}()
	}

	<-started
	// 给其余 goroutine 时间撞上 refreshing 标志
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	mu.Lock()
	calls := tableCalls
	mu.Unlock()
	if calls != 1 {
		t.Fatalf("concurrent refreshes = %d, want 1 (single-flight)", calls)
	}
}
