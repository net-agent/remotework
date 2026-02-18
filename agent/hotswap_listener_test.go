package agent

import (
	"errors"
	"net"
	"testing"
)

func TestHotSwapListener_Refresh(t *testing.T) {
	callCount := 0
	hsl := NewHotSwapListener(func() (net.Listener, error) {
		callCount++
		return newMockListener("mock:0"), nil
	})

	if err := hsl.Refresh(); err != nil {
		t.Fatalf("first Refresh() error: %v", err)
	}
	if hsl.Get() == nil {
		t.Fatal("Get() returned nil after Refresh")
	}
	if callCount != 1 {
		t.Errorf("listenFn called %d times, want 1", callCount)
	}
}

func TestHotSwapListener_RefreshReplacesOld(t *testing.T) {
	listeners := make([]*mockListener, 0)
	hsl := NewHotSwapListener(func() (net.Listener, error) {
		l := newMockListener("mock:0")
		listeners = append(listeners, l)
		return l, nil
	})

	hsl.Refresh()
	first := listeners[0]

	hsl.Refresh()
	second := listeners[1]

	// 旧 listener 应该被关闭
	if !first.isClosed() {
		t.Error("first listener should be closed after replacement")
	}
	// 新 listener 应该是当前的
	if hsl.Get() != second {
		t.Error("Get() should return the second listener")
	}
	if second.isClosed() {
		t.Error("second listener should not be closed")
	}
}

func TestHotSwapListener_RefreshError(t *testing.T) {
	first := newMockListener("mock:0")
	callCount := 0
	hsl := NewHotSwapListener(func() (net.Listener, error) {
		callCount++
		if callCount == 1 {
			return first, nil
		}
		return nil, errors.New("listen failed")
	})

	// 第一次成功
	if err := hsl.Refresh(); err != nil {
		t.Fatalf("first Refresh() error: %v", err)
	}

	// 第二次失败，不应影响旧 listener
	if err := hsl.Refresh(); err == nil {
		t.Fatal("second Refresh() should fail")
	}
	if first.isClosed() {
		t.Error("original listener should NOT be closed on Refresh failure")
	}
	if hsl.Get() != first {
		t.Error("Get() should still return original listener after failed Refresh")
	}
}

func TestHotSwapListener_Close(t *testing.T) {
	ml := newMockListener("mock:0")
	hsl := NewHotSwapListener(func() (net.Listener, error) {
		return ml, nil
	})
	hsl.Refresh()

	if err := hsl.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if !ml.isClosed() {
		t.Error("listener should be closed")
	}
	if hsl.Get() != nil {
		t.Error("Get() should return nil after Close")
	}

	// 幂等：再次 Close 不应报错
	if err := hsl.Close(); err != nil {
		t.Errorf("second Close() error: %v", err)
	}
}

func TestHotSwapListener_GetBeforeRefresh(t *testing.T) {
	hsl := NewHotSwapListener(func() (net.Listener, error) {
		return newMockListener("mock:0"), nil
	})
	if hsl.Get() != nil {
		t.Error("Get() before Refresh should return nil")
	}
}
