package kernel_test

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

// TestPendingDependencyTimeout Pending 等待依赖超时后转 Failed，错误信息明确；
// 超时后 Retry 仍然可用（无旧回调在运行）。
func TestPendingDependencyTimeout(t *testing.T) {
	h := newTestHost(t, &kernel.HostOptions{DependencyTimeout: 100 * time.Millisecond})

	waiter := &fakePlugin{
		info: kernel.Info{Name: "waiter", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			_, err := kernel.Inject[greetCap](c, "late")
			return err
		},
	}
	if err := h.Load(waiter); err != nil {
		t.Fatalf("load: %v", err)
	}
	waitState(t, h, "waiter", kernel.StatePending)

	st := waitState(t, h, "waiter", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "timed out") {
		t.Fatalf("expect dependency timeout error, got %q", st.LastErr)
	}
	if !strings.Contains(st.LastErr, `"late"`) {
		t.Fatalf("timeout error should name the missing capability, got %q", st.LastErr)
	}

	// 没有仍在运行的旧回调：Retry 应被接受，重新 Init 后回到 Pending
	if err := h.Retry("waiter"); err != nil {
		t.Fatalf("retry after dependency timeout: %v", err)
	}
	waitState(t, h, "waiter", kernel.StatePending)
}

// TestPendingAutoRetryBeforeTimeout 依赖在超时前出现时，正常自动重试不受影响；
// 已 Running 的插件也不会被残留定时器误判为超时。
func TestPendingAutoRetryBeforeTimeout(t *testing.T) {
	h := newTestHost(t, &kernel.HostOptions{DependencyTimeout: 200 * time.Millisecond})

	waiter := &fakePlugin{
		info: kernel.Info{Name: "waiter", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			_, err := kernel.Inject[greetCap](c, "late")
			return err
		},
	}
	if err := h.Load(waiter); err != nil {
		t.Fatalf("load: %v", err)
	}
	waitState(t, h, "waiter", kernel.StatePending)

	h.Load(&fakePlugin{
		info:   kernel.Info{Name: "provider"},
		initFn: provideGreet("late"),
	})
	waitState(t, h, "waiter", kernel.StateRunning)

	// 越过原 Pending 起点 + DependencyTimeout，仍应保持 Running
	time.Sleep(250 * time.Millisecond)
	st, err := h.State("waiter")
	if err != nil || st.State != kernel.StateRunning {
		t.Fatalf("running plugin must not be failed by dependency timer: state=%s err=%v", st.State, err)
	}
}

// TestPendingStartTimeoutRetryKeepsInit Start 阶段进入 Pending 后超时，
// Retry 应只重跑 Start（不重新 Init）。
func TestPendingStartTimeoutRetryKeepsInit(t *testing.T) {
	h := newTestHost(t, &kernel.HostOptions{DependencyTimeout: 100 * time.Millisecond})

	var initCount, startCount atomic.Int32
	p := &fakePlugin{
		info: kernel.Info{Name: "slow-start", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			initCount.Add(1)
			return nil
		},
		startFn: func(c *kernel.Context) error {
			startCount.Add(1)
			_, err := kernel.Inject[greetCap](c, "late")
			return err
		},
	}
	if err := h.Load(p); err != nil {
		t.Fatalf("load: %v", err)
	}
	waitState(t, h, "slow-start", kernel.StatePending)
	st := waitState(t, h, "slow-start", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "timed out") {
		t.Fatalf("expect dependency timeout error, got %q", st.LastErr)
	}

	if err := h.Retry("slow-start"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitState(t, h, "slow-start", kernel.StatePending)
	if got := initCount.Load(); got != 1 {
		t.Fatalf("retry after Start pending timeout must not rerun Init, initCount=%d", got)
	}
	if got := startCount.Load(); got != 2 {
		t.Fatalf("Start should be retried once, startCount=%d", got)
	}
}
