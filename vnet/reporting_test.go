package vnet

import (
	"sync"
	"testing"
)

func TestReportingNetwork_Report_Basic(t *testing.T) {
	mn := NewMockNetwork("test-net")
	mn.meta = &NetworkMeta{Protocol: "tcp", Address: "1.2.3.4:80", Domain: "d1", State: "online"}
	rn := NewReportingNetwork(mn)

	report := rn.Report()
	if report.Name != "test-net" {
		t.Errorf("Name = %q, want %q", report.Name, "test-net")
	}
	if report.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", report.Protocol, "tcp")
	}
	if report.State != "online" {
		t.Errorf("State = %q, want %q", report.State, "online")
	}
	if report.Dials != 0 {
		t.Errorf("Dials = %d, want 0", report.Dials)
	}
}

func TestReportingNetwork_CountsDialAndListen(t *testing.T) {
	mn := NewMockNetwork("test-net")
	rn := NewReportingNetwork(mn)

	// Dial/Listen 会失败（mock 默认返回 error），但计数器仍应递增
	rn.Dial("tcp", "1.2.3.4:80")
	rn.Dial("tcp", "1.2.3.4:81")
	rn.Listen("test-net", "0:80")

	report := rn.Report()
	if report.Dials != 2 {
		t.Errorf("Dials = %d, want 2", report.Dials)
	}
	if report.Listens != 1 {
		t.Errorf("Listens = %d, want 1", report.Listens)
	}
}

func TestReportingNetwork_NoMetaProvider(t *testing.T) {
	// mockNetwork without meta set — still implements NetworkMetaProvider, returns empty
	mn := NewMockNetwork("bare")
	rn := NewReportingNetwork(mn)

	report := rn.Report()
	if report.Name != "bare" {
		t.Errorf("Name = %q, want %q", report.Name, "bare")
	}
	if report.Alive < 0 {
		t.Error("Alive should be non-negative")
	}
}

func TestReportingNetwork_ConcurrentAccess(t *testing.T) {
	mn := NewMockNetwork("concurrent")
	rn := NewReportingNetwork(mn)
	var wg sync.WaitGroup
	n := 100

	wg.Add(n * 2)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			rn.Dial("tcp", "0:80")
		}()
		go func() {
			defer wg.Done()
			rn.Listen("concurrent", "0:80")
		}()
	}
	wg.Wait()

	report := rn.Report()
	if report.Dials != int32(n) {
		t.Errorf("Dials = %d, want %d", report.Dials, n)
	}
	if report.Listens != int32(n) {
		t.Errorf("Listens = %d, want %d", report.Listens, n)
	}
}
