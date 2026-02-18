package agent

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()
	if hub.Networks == nil {
		t.Fatal("Networks is nil")
	}
	if hub.Services == nil {
		t.Fatal("Services is nil")
	}

	// NewHub 应该包含 tcp 网络
	for _, name := range []string{"tcp", "tcp4", "tcp6"} {
		if _, err := hub.Networks.Find(name); err != nil {
			t.Errorf("NewHub missing network %q: %v", name, err)
		}
	}
}

func TestHub_AddAndFindNetwork(t *testing.T) {
	hub := NewHub()
	mn := newMockNetwork("vnet")

	if err := hub.AddNetwork(mn); err != nil {
		t.Fatalf("AddNetwork() error: %v", err)
	}

	got, err := hub.FindNetwork("vnet")
	if err != nil {
		t.Fatalf("FindNetwork() error: %v", err)
	}
	if got != mn {
		t.Error("FindNetwork() returned different network")
	}
}

func TestHub_AddAndFindService(t *testing.T) {
	hub := NewHub()
	svc, _ := newTestService("svc1", "portproxy", "tcp://0:80", "tcp://0:81")

	if err := hub.AddService(svc); err != nil {
		t.Fatalf("AddService() error: %v", err)
	}

	got, err := hub.FindService("svc1")
	if err != nil {
		t.Fatalf("FindService() error: %v", err)
	}
	if got != svc {
		t.Error("FindService() returned different service")
	}
}

func TestHub_UpdateNetwork(t *testing.T) {
	hub := NewHub()
	svc, ctrl := newTestService("pp", "portproxy", "vtcp://host:80", "tcp://0:81")
	hub.AddService(svc)
	svc.SetStatus(StatusRunning)

	hub.UpdateNetwork("vtcp://")

	// UpdateByNetwork 内部用 go svc.controller.Update()，等一下
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&ctrl.updateCalled) != 1 {
		t.Errorf("Update called %d times, want 1", atomic.LoadInt32(&ctrl.updateCalled))
	}
}

func TestHub_Dial(t *testing.T) {
	hub := NewHub()
	mn := newMockNetwork("mynet")
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mn.dialFn = func(network, addr string) (net.Conn, error) {
		return c1, nil
	}
	hub.AddNetwork(mn)

	conn, err := hub.Dial("mynet", "host:80")
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	if conn != c1 {
		t.Error("Dial() returned unexpected conn")
	}
}

func TestHub_ListenURL(t *testing.T) {
	hub := NewHub()
	l, err := hub.ListenURL("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenURL() error: %v", err)
	}
	l.Close()
}

func TestHub_URLDialer(t *testing.T) {
	hub := NewHub()
	mn := newMockNetwork("mynet")
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mn.dialFn = func(network, addr string) (net.Conn, error) {
		return c1, nil
	}
	hub.AddNetwork(mn)

	dialer, err := hub.URLDialer("mynet://host:80")
	if err != nil {
		t.Fatalf("URLDialer() error: %v", err)
	}
	conn, err := dialer()
	if err != nil {
		t.Fatalf("dialer() error: %v", err)
	}
	if conn != c1 {
		t.Error("dialer() returned unexpected conn")
	}
}

func TestHub_IsRunning(t *testing.T) {
	hub := NewHub()
	if hub.IsRunning() {
		t.Error("new Hub should not be running")
	}
}

func TestHub_RangeAllService(t *testing.T) {
	hub := NewHub()
	svc1, _ := newTestService("a", "portproxy", "", "")
	svc2, _ := newTestService("b", "socks5", "", "")
	hub.AddService(svc1)
	hub.AddService(svc2)

	var names []string
	hub.RangeAllService(func(svc *Service) {
		names = append(names, svc.Name)
	})
	if len(names) != 2 {
		t.Errorf("RangeAllService visited %d, want 2", len(names))
	}
}

func TestHub_StopServices_StopNetworks(t *testing.T) {
	hub := NewHub()
	mn := newMockNetwork("vnet")
	hub.AddNetwork(mn)

	// StopNetworks 应该调用所有网络的 Stop
	hub.StopNetworks()
	if !mn.isStopped() {
		t.Error("StopNetworks should stop added networks")
	}
}
