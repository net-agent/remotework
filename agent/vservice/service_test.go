package vservice

import (
	"errors"
	"fmt"
	"testing"
)

func TestServiceStatus_String(t *testing.T) {
	tests := []struct {
		status ServiceStatus
		want   string
	}{
		{StatusUninit, "uninit"},
		{StatusInit, "init"},
		{StatusRunning, "running"},
		{StatusStopped, "stopped"},
		{StatusFailed, "init failed"},
		{StatusPending, "pending"},
		{ServiceStatus(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("ServiceStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestServiceState_SetGetStatus(t *testing.T) {
	s := &ServiceState{}
	if got := s.GetStatus(); got != StatusUninit {
		t.Fatalf("initial status = %v, want StatusUninit", got)
	}
	s.SetStatus(StatusRunning)
	if got := s.GetStatus(); got != StatusRunning {
		t.Errorf("status = %v, want StatusRunning", got)
	}
}

func TestServiceState_StatusString(t *testing.T) {
	s := &ServiceState{}
	s.SetStatus(StatusFailed)
	if got := s.StatusString(); got != "init failed" {
		t.Errorf("StatusString() = %q, want %q", got, "init failed")
	}
}

func TestServiceState_ActiveAndDoneCount(t *testing.T) {
	s := &ServiceState{}
	s.AddActiveCount(3)
	if got := s.GetActiveCount(); got != 3 {
		t.Fatalf("active count = %d, want 3", got)
	}
	if got := s.GetDoneCount(); got != 0 {
		t.Fatalf("done count = %d, want 0", got)
	}

	// AddDoneCount decrements actives and increments dones
	s.AddDoneCount(2)
	if got := s.GetActiveCount(); got != 1 {
		t.Errorf("active count after done = %d, want 1", got)
	}
	if got := s.GetDoneCount(); got != 2 {
		t.Errorf("done count = %d, want 2", got)
	}
}

func TestServiceState_IsListenDepend(t *testing.T) {
	s := &ServiceState{ListenURL: "vtcp://myhost:8080?secret=abc"}

	if !s.IsListenDepend("vtcp://") {
		t.Error("expected IsListenDepend(vtcp://) = true")
	}
	if !s.IsListenDepend("vtcp://myhost") {
		t.Error("expected IsListenDepend(vtcp://myhost) = true")
	}
	if s.IsListenDepend("tcp://") {
		t.Error("expected IsListenDepend(tcp://) = false")
	}
	if s.IsListenDepend("ws://") {
		t.Error("expected IsListenDepend(ws://) = false")
	}
}

func TestServiceState_IsTargetDepend(t *testing.T) {
	s := &ServiceState{TargetURL: "vtcp://remote:3389?secret=key"}

	if !s.IsTargetDepend("vtcp://") {
		t.Error("expected IsTargetDepend(vtcp://) = true")
	}
	if s.IsTargetDepend("tcp://") {
		t.Error("expected IsTargetDepend(tcp://) = false")
	}
}

func TestServiceState_IsDepend(t *testing.T) {
	s := &ServiceState{
		ListenURL: "vtcp://host:80?secret=abc",
		TargetURL: "tcp://localhost:3389",
	}

	if !s.IsDepend("vtcp://") {
		t.Error("expected IsDepend(vtcp://) = true via ListenURL")
	}
	if !s.IsDepend("tcp://") {
		t.Error("expected IsDepend(tcp://) = true via TargetURL")
	}
	if s.IsDepend("ws://") {
		t.Error("expected IsDepend(ws://) = false")
	}
}

func TestErrDependencyNotReady_ViaVnet(t *testing.T) {
	// ErrDependencyNotReady 通过 errors.As 应该正常工作
	err := fmt.Errorf("init: %w", &errDepWrap{Network: "vtcpx"})
	var depErr *errDepWrap
	if !errors.As(err, &depErr) {
		t.Error("errors.As should match wrapped error")
	}
	if depErr.Network != "vtcpx" {
		t.Errorf("Network = %q, want %q", depErr.Network, "vtcpx")
	}
}

// errDepWrap 用于在 vservice 测试中模拟 ErrDependencyNotReady
type errDepWrap struct {
	Network string
}

func (e *errDepWrap) Error() string {
	return fmt.Sprintf("dependency network '%s' not ready", e.Network)
}

func TestNewPortproxyService(t *testing.T) {
	lf := &mockListenerFactory{}
	df := &mockDialerFactory{}
	svc := NewPortproxyService(lf, df, "pp-test", "vtcp://0:1234", "tcp://localhost:3389")

	if svc.Type != "portproxy" {
		t.Errorf("Type = %q, want %q", svc.Type, "portproxy")
	}
	if svc.Name != "pp-test" {
		t.Errorf("Name = %q, want %q", svc.Name, "pp-test")
	}
	if svc.ListenURL != "vtcp://0:1234" {
		t.Errorf("ListenURL = %q, want %q", svc.ListenURL, "vtcp://0:1234")
	}
	if svc.TargetURL != "tcp://localhost:3389" {
		t.Errorf("TargetURL = %q, want %q", svc.TargetURL, "tcp://localhost:3389")
	}
	if svc.Controller == nil {
		t.Error("Controller is nil")
	}
}

func TestNewPortproxyService_DefaultName(t *testing.T) {
	svc := NewPortproxyService(&mockListenerFactory{}, &mockDialerFactory{}, "", "tcp://0:80", "tcp://0:81")
	if svc.Name != "portproxy" {
		t.Errorf("default Name = %q, want %q", svc.Name, "portproxy")
	}
}

func TestNewSocks5Service(t *testing.T) {
	lf := &mockListenerFactory{}
	svc := NewSocks5Service(lf, "sox-test", "vtcp://0:1080", "user", "pass")

	if svc.Type != "socks5" {
		t.Errorf("Type = %q, want %q", svc.Type, "socks5")
	}
	if svc.Name != "sox-test" {
		t.Errorf("Name = %q, want %q", svc.Name, "sox-test")
	}
	if svc.Username != "user" || svc.Password != "pass" {
		t.Errorf("credentials = %q/%q, want user/pass", svc.Username, svc.Password)
	}
	if svc.Controller == nil {
		t.Error("Controller is nil")
	}
}

