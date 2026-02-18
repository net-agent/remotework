package agent

import (
	"sync"
	"testing"
)

func TestNetworkinfo_GetName(t *testing.T) {
	info := networkinfo{name: "test-net"}
	if got := info.GetName(); got != "test-net" {
		t.Errorf("GetName() = %q, want %q", got, "test-net")
	}
}

func TestNetworkinfo_DialCount(t *testing.T) {
	info := networkinfo{}
	if got := info.getDialCount(); got != 0 {
		t.Fatalf("initial dial count = %d, want 0", got)
	}
	info.addDialCount(1)
	info.addDialCount(3)
	if got := info.getDialCount(); got != 4 {
		t.Errorf("dial count = %d, want 4", got)
	}
}

func TestNetworkinfo_ListenCount(t *testing.T) {
	info := networkinfo{}
	if got := info.getListenCount(); got != 0 {
		t.Fatalf("initial listen count = %d, want 0", got)
	}
	info.addListenCount(2)
	info.addListenCount(5)
	if got := info.getListenCount(); got != 7 {
		t.Errorf("listen count = %d, want 7", got)
	}
}

func TestNetworkinfo_ConcurrentAccess(t *testing.T) {
	info := networkinfo{}
	var wg sync.WaitGroup
	n := 100

	wg.Add(n * 2)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			info.addDialCount(1)
		}()
		go func() {
			defer wg.Done()
			info.addListenCount(1)
		}()
	}
	wg.Wait()

	if got := info.getDialCount(); got != int32(n) {
		t.Errorf("concurrent dial count = %d, want %d", got, n)
	}
	if got := info.getListenCount(); got != int32(n) {
		t.Errorf("concurrent listen count = %d, want %d", got, n)
	}
}
