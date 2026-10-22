package kernel_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

// TestInitTimeoutAttemptIsolation 覆盖 P1-2：
//   - Init 超时后旧回调仍在运行，Retry 必须被 ErrAttemptRunning 拒绝；
//   - 旧回调结束后才允许 Retry；
//   - 旧 attempt 的 DeferCleanup 在回调结束后回滚，不会混入新 attempt；
//   - 超时后迟到的 Provide/DeferCleanup 失败。
func TestInitTimeoutAttemptIsolation(t *testing.T) {
	block := make(chan struct{})
	var unblockOnce sync.Once
	unblock := func() { unblockOnce.Do(func() { close(block) }) }
	t.Cleanup(unblock)

	h := newTestHost(t, &kernel.HostOptions{InitTimeout: 50 * time.Millisecond})
	rec := &recorder{}
	var tries atomic.Int32
	lateCleanupErr := make(chan error, 1)
	lateProvideErr := make(chan error, 1)

	p := &fakePlugin{
		info: kernel.Info{Name: "slow", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			if tries.Add(1) == 1 {
				if err := c.DeferCleanup(func(ctx context.Context) error {
					rec.add("old.cleanup")
					return nil
				}); err != nil {
					return err
				}
				<-block
				// 超时已发生：phase 已被封锁，迟到写入必须失败
				lateCleanupErr <- c.DeferCleanup(func(ctx context.Context) error {
					rec.add("late.cleanup")
					return nil
				})
				lateProvideErr <- kernel.Provide[greetCap](c, "late-cap", greetImpl("late"))
				return nil
			}
			rec.add("new.init")
			return c.DeferCleanup(func(ctx context.Context) error {
				rec.add("new.cleanup")
				return nil
			})
		},
	}
	if err := h.Load(p); err != nil {
		t.Fatalf("load: %v", err)
	}

	st := waitState(t, h, "slow", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "timeout") {
		t.Fatalf("expect timeout error, got %q", st.LastErr)
	}

	// 旧回调仍阻塞在 Init 中：立即 Retry 必须被明确拒绝
	err := h.Retry("slow")
	if !errors.Is(err, kernel.ErrAttemptRunning) {
		t.Fatalf("retry while callback running: got %v, want ErrAttemptRunning", err)
	}

	unblock()

	// 等旧回调返回、回滚完成并解除 attemptRunning
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = h.Retry("slow")
		if err == nil {
			break
		}
		if !errors.Is(err, kernel.ErrAttemptRunning) {
			t.Fatalf("unexpected retry error: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("retry still rejected after old callback returned")
		}
		time.Sleep(5 * time.Millisecond)
	}
	waitState(t, h, "slow", kernel.StateRunning)

	if err := <-lateCleanupErr; err == nil {
		t.Fatal("DeferCleanup after attempt timeout should fail")
	}
	if err := <-lateProvideErr; err == nil {
		t.Fatal("Provide after attempt timeout should fail")
	}
	if rec.index("late.cleanup") >= 0 {
		t.Fatal("late cleanup registered after timeout must never run")
	}
	if !rec.before("old.cleanup", "new.init") {
		t.Fatalf("old attempt rollback must finish before new attempt starts, got %v", rec.ev)
	}

	if err := h.Unload("slow", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if got := rec.count("old.cleanup"); got != 1 {
		t.Fatalf("old cleanup should run exactly once (rollback), got %d; events=%v", got, rec.ev)
	}
	if got := rec.count("new.cleanup"); got != 1 {
		t.Fatalf("new cleanup should run exactly once (unload), got %d; events=%v", got, rec.ev)
	}
}

// TestStartTimeoutAttemptIsolation Start 超时同样隔离旧回调；Retry 只重跑 Start，
// 旧 Start 的暂存 cleanup 在回调结束后回滚，不进入新 attempt。
func TestStartTimeoutAttemptIsolation(t *testing.T) {
	block := make(chan struct{})
	var unblockOnce sync.Once
	unblock := func() { unblockOnce.Do(func() { close(block) }) }
	t.Cleanup(unblock)

	h := newTestHost(t, &kernel.HostOptions{StartTimeout: 50 * time.Millisecond})
	rec := &recorder{}
	var initCount, startCount atomic.Int32
	lateCleanupErr := make(chan error, 1)

	p := &fakePlugin{
		info: kernel.Info{Name: "slowstart", Kind: "test"},
		initFn: func(c *kernel.Context) error {
			initCount.Add(1)
			return c.DeferCleanup(func(ctx context.Context) error {
				rec.add("init.cleanup")
				return nil
			})
		},
		startFn: func(c *kernel.Context) error {
			if startCount.Add(1) == 1 {
				if err := c.DeferCleanup(func(ctx context.Context) error {
					rec.add("old.start.cleanup")
					return nil
				}); err != nil {
					return err
				}
				<-block
				lateCleanupErr <- c.DeferCleanup(func(ctx context.Context) error {
					rec.add("late.start.cleanup")
					return nil
				})
				return nil
			}
			return c.DeferCleanup(func(ctx context.Context) error {
				rec.add("new.start.cleanup")
				return nil
			})
		},
	}
	if err := h.Load(p); err != nil {
		t.Fatalf("load: %v", err)
	}

	st := waitState(t, h, "slowstart", kernel.StateFailed)
	if !strings.Contains(st.LastErr, "timeout") {
		t.Fatalf("expect timeout error, got %q", st.LastErr)
	}
	if err := h.Retry("slowstart"); !errors.Is(err, kernel.ErrAttemptRunning) {
		t.Fatalf("retry while Start callback running: got %v, want ErrAttemptRunning", err)
	}

	unblock()

	deadline := time.Now().Add(3 * time.Second)
	for {
		err := h.Retry("slowstart")
		if err == nil {
			break
		}
		if !errors.Is(err, kernel.ErrAttemptRunning) {
			t.Fatalf("unexpected retry error: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("retry still rejected after old Start callback returned")
		}
		time.Sleep(5 * time.Millisecond)
	}
	waitState(t, h, "slowstart", kernel.StateRunning)

	if initCount.Load() != 1 {
		t.Fatalf("Start timeout retry must not rerun Init, initCount=%d", initCount.Load())
	}
	if startCount.Load() != 2 {
		t.Fatalf("Start should run twice, startCount=%d", startCount.Load())
	}
	if err := <-lateCleanupErr; err == nil {
		t.Fatal("DeferCleanup after Start timeout should fail")
	}
	if rec.index("late.start.cleanup") >= 0 {
		t.Fatal("late cleanup registered after timeout must never run")
	}

	if err := h.Unload("slowstart", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if got := rec.count("old.start.cleanup"); got != 1 {
		t.Fatalf("old Start cleanup should run exactly once (rollback), got %d; events=%v", got, rec.ev)
	}
	if got := rec.count("new.start.cleanup"); got != 1 {
		t.Fatalf("new Start cleanup should run exactly once (unload), got %d; events=%v", got, rec.ev)
	}
	if got := rec.count("init.cleanup"); got != 1 {
		t.Fatalf("committed Init cleanup should run exactly once, got %d; events=%v", got, rec.ev)
	}
}
