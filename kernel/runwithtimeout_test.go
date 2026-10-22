package kernel

import (
	"testing"
	"time"
)

// TestRunWithTimeoutFinishedContract 锁定 runWithTimeout 的 finished 契约：
// 正常完成时返回前已关闭；超时返回时回调仍在运行，finished 尚未关闭。
func TestRunWithTimeoutFinishedContract(t *testing.T) {
	if err, finished, timedOut := runWithTimeout(time.Second, func() error { return nil }); err != nil || timedOut {
		t.Fatalf("normal callback should not fail/timeout: err=%v timedOut=%v", err, timedOut)
	} else {
		select {
		case <-finished:
		default:
			t.Fatal("finished must already be closed on normal completion")
		}
	}

	release := make(chan struct{})
	err, finished, timedOut := runWithTimeout(20*time.Millisecond, func() error {
		<-release
		return nil
	})
	if err == nil || !timedOut {
		t.Fatalf("blocking callback should time out: err=%v timedOut=%v", err, timedOut)
	}
	select {
	case <-finished:
		t.Fatal("finished must stay open while callback is still running")
	default:
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("finished was not closed after callback returned")
	}
}
