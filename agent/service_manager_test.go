package agent

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestServiceManager_AddAndFind(t *testing.T) {
	sm := NewServiceManager(nil)
	svc, _ := newTestService("svc1", "portproxy", "tcp://0:80", "tcp://0:81")

	if err := sm.Add(svc); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if svc.ID == 0 {
		t.Error("ID should be assigned after Add")
	}

	got, err := sm.Find("svc1")
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}
	if got != svc {
		t.Error("Find() returned different service")
	}
}

func TestServiceManager_AddDuplicate(t *testing.T) {
	sm := NewServiceManager(nil)
	svc1, _ := newTestService("dup", "portproxy", "", "")
	svc2, _ := newTestService("dup", "socks5", "", "")

	sm.Add(svc1)
	err := sm.Add(svc2)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want contains 'duplicate'", err.Error())
	}
}

func TestServiceManager_FindNotExist(t *testing.T) {
	sm := NewServiceManager(nil)
	_, err := sm.Find("nope")
	if err == nil {
		t.Fatal("expected error for non-existent service")
	}
}

func TestServiceManager_Range(t *testing.T) {
	sm := NewServiceManager(nil)
	svc1, _ := newTestService("a", "portproxy", "", "")
	svc2, _ := newTestService("b", "socks5", "", "")
	sm.Add(svc1)
	sm.Add(svc2)

	var visited []string
	sm.Range(func(svc *Service) {
		visited = append(visited, svc.Name)
	})
	if len(visited) != 2 {
		t.Fatalf("Range visited %d services, want 2", len(visited))
	}
}

func TestServiceManager_StartAll(t *testing.T) {
	sm := NewServiceManager(nil)
	svc, ctrl := newTestService("svc1", "portproxy", "", "")
	ctrl.startBlock = make(chan struct{})
	sm.Add(svc)

	done := make(chan error, 1)
	go func() {
		done <- sm.StartAll()
	}()

	// 等待服务进入 Running 状态
	deadline := time.After(2 * time.Second)
	for {
		if svc.GetStatus() == StatusRunning {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for service to reach Running status")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if atomic.LoadInt32(&ctrl.initCalled) != 1 {
		t.Error("Init should be called once")
	}
	if atomic.LoadInt32(&ctrl.startCalled) != 1 {
		t.Error("Start should be called once")
	}

	// 解除 Start 阻塞
	close(ctrl.startBlock)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("StartAll() error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StartAll() did not return")
	}

	if svc.GetStatus() != StatusStopped {
		t.Errorf("status = %v, want StatusStopped", svc.GetStatus())
	}
}

func TestServiceManager_StartAll_RejectDouble(t *testing.T) {
	sm := NewServiceManager(nil)
	svc, ctrl := newTestService("svc1", "portproxy", "", "")
	ctrl.startBlock = make(chan struct{})
	sm.Add(svc)

	go sm.StartAll()

	// 等待 running 标志位
	deadline := time.After(2 * time.Second)
	for {
		if sm.IsRunning() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for IsRunning")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	err := sm.StartAll()
	if err == nil {
		t.Fatal("second StartAll() should return error")
	}

	close(ctrl.startBlock)
}

func TestServiceManager_StopAll(t *testing.T) {
	sm := NewServiceManager(nil)
	svc1, ctrl1 := newTestService("running-svc", "portproxy", "", "")
	svc2, ctrl2 := newTestService("stopped-svc", "portproxy", "", "")

	sm.Add(svc1)
	sm.Add(svc2)

	// 模拟 svc1 处于 Running 状态
	svc1.SetStatus(StatusRunning)
	svc2.SetStatus(StatusStopped)

	// StopAll 需要 running 标志位为 1
	atomic.StoreInt32(&sm.running, 1)

	sm.StopAll()

	if atomic.LoadInt32(&ctrl1.closeCalled) != 1 {
		t.Error("running service should have Close called")
	}
	if atomic.LoadInt32(&ctrl2.closeCalled) != 0 {
		t.Error("stopped service should NOT have Close called")
	}
}

func TestServiceManager_StopAll_NotRunning(t *testing.T) {
	sm := NewServiceManager(nil)
	svc, ctrl := newTestService("svc", "portproxy", "", "")
	svc.SetStatus(StatusRunning)
	sm.Add(svc)

	// running == 0, StopAll 应该直接返回
	sm.StopAll()
	if atomic.LoadInt32(&ctrl.closeCalled) != 0 {
		t.Error("StopAll should be no-op when not running")
	}
}

func TestServiceManager_IsRunning(t *testing.T) {
	sm := NewServiceManager(nil)
	if sm.IsRunning() {
		t.Error("new ServiceManager should not be running")
	}
}

func TestServiceManager_Names(t *testing.T) {
	sm := NewServiceManager(nil)
	svc1, _ := newTestService("alpha", "portproxy", "", "")
	svc2, _ := newTestService("beta", "socks5", "", "")
	sm.Add(svc1)
	sm.Add(svc2)

	names := sm.Names()
	if len(names) != 2 {
		t.Fatalf("Names() len = %d, want 2", len(names))
	}
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["alpha"] || !nameSet["beta"] {
		t.Errorf("Names() = %v, want [alpha, beta]", names)
	}
}

func TestServiceManager_StartAll_AllInitFailed(t *testing.T) {
	sm := NewServiceManager(nil)
	svc1, ctrl1 := newTestService("fail1", "portproxy", "", "")
	svc2, ctrl2 := newTestService("fail2", "socks5", "", "")
	ctrl1.initErr = errors.New("boom1")
	ctrl2.initErr = errors.New("boom2")
	sm.Add(svc1)
	sm.Add(svc2)

	err := sm.StartAll()
	if err == nil {
		t.Fatal("StartAll() should return error when all services fail init")
	}
	if !strings.Contains(err.Error(), "2/2") {
		t.Errorf("error = %q, want contains '2/2'", err.Error())
	}
}

func TestServiceManager_StartAll_PartialInitFailed(t *testing.T) {
	sm := NewServiceManager(nil)
	svc1, ctrl1 := newTestService("ok-svc", "portproxy", "", "")
	svc2, ctrl2 := newTestService("fail-svc", "socks5", "", "")
	ctrl1.startBlock = make(chan struct{})
	ctrl2.initErr = errors.New("boom")
	sm.Add(svc1)
	sm.Add(svc2)

	done := make(chan error, 1)
	go func() {
		done <- sm.StartAll()
	}()

	// 等待 ok-svc 进入 Running
	deadline := time.After(2 * time.Second)
	for {
		if svc1.GetStatus() == StatusRunning {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for ok-svc to reach Running")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// 解除 ok-svc 的 Start 阻塞
	close(ctrl1.startBlock)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("StartAll() should return error when some services fail init")
		}
		if !strings.Contains(err.Error(), "1/2") {
			t.Errorf("error = %q, want contains '1/2'", err.Error())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StartAll() did not return")
	}

	_ = ctrl2 // used for initErr setup
}

func TestServiceManager_InitDependencyNotReady_SetsPending(t *testing.T) {
	sm := NewServiceManager(nil)
	svc, ctrl := newTestService("dep-svc", "portproxy", "vtcpx://host:80", "tcp://localhost:3389")
	ctrl.initErr = &ErrDependencyNotReady{Network: "vtcpx"}
	sm.Add(svc)

	err := sm.StartAll()
	if err != nil {
		t.Fatalf("StartAll() error: %v (pending should not count as failed)", err)
	}

	if svc.GetStatus() != StatusPending {
		t.Errorf("status = %v, want StatusPending", svc.GetStatus())
	}
}

