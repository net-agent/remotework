package agent

import (
	"net"
	"strings"
	"testing"
)

func TestNetworkRegistry_AddAndFind(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	mn := newMockNetwork("test-net")

	if err := nr.Add(mn); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	got, err := nr.Find("test-net")
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}
	if got != mn {
		t.Error("Find() returned different network")
	}
}

func TestNetworkRegistry_AddDuplicate(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	nr.Add(newMockNetwork("dup"))

	err := nr.Add(newMockNetwork("dup"))
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
	if !strings.Contains(err.Error(), "exists") {
		t.Errorf("error = %q, want contains 'exists'", err.Error())
	}
}

func TestNetworkRegistry_AddEmptyName(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	err := nr.Add(newMockNetwork(""))
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNetworkRegistry_FindNotExist(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	_, err := nr.Find("nope")
	if err == nil {
		t.Fatal("expected error for non-existent network")
	}
}

func TestNetworkRegistry_FindEmptyName(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	_, err := nr.Find("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNetworkRegistry_IsPrivateNetwork(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	nr.Add(newMockNetwork("vnet"))

	tests := []struct {
		name string
		want bool
	}{
		{"vnet", true},
		{"tcp", false},
		{"tcp4", false},
		{"tcp6", false},
		{"", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := nr.IsPrivateNetwork(tt.name); got != tt.want {
			t.Errorf("IsPrivateNetwork(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestNetworkRegistry_Dial(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	mn := newMockNetwork("mynet")
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mn.dialFn = func(network, addr string) (net.Conn, error) {
		return c1, nil
	}
	nr.Add(mn)

	conn, err := nr.Dial("mynet", "host:80")
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	if conn != c1 {
		t.Error("Dial() returned unexpected conn")
	}
}

func TestNetworkRegistry_Dial_NotFound(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	_, err := nr.Dial("nope", "host:80")
	if err == nil {
		t.Fatal("expected error for unknown network")
	}
}

func TestNetworkRegistry_Listen(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	mn := newMockNetwork("mynet")
	ml := newMockListener("mock:1234")
	mn.listenFn = func(network, addr string) (net.Listener, error) {
		return ml, nil
	}
	nr.Add(mn)

	l, err := nr.Listen("mynet", "0:1234")
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	if l != ml {
		t.Error("Listen() returned unexpected listener")
	}
}

func TestNetworkRegistry_ListenURL_TCP(t *testing.T) {
	nr := NewNetworkRegistry(nil) // 包含 tcp

	l, err := nr.ListenURL("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenURL() error: %v", err)
	}
	l.Close()
}

func TestNetworkRegistry_ListenURL_PrivateNoSecret(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	mn := newMockNetwork("vnet")
	mn.listenFn = func(network, addr string) (net.Listener, error) {
		return newMockListener("mock:0"), nil
	}
	nr.Add(mn)

	_, err := nr.ListenURL("vnet://0:1234")
	if err == nil {
		t.Fatal("expected error for private network without secret")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Errorf("error = %q, want contains 'secret'", err.Error())
	}
}

func TestNetworkRegistry_URLDialer(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	mn := newMockNetwork("mynet")
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mn.dialFn = func(network, addr string) (net.Conn, error) {
		return c1, nil
	}
	nr.Add(mn)

	dialer, err := nr.URLDialer("mynet://host:80")
	if err != nil {
		t.Fatalf("URLDialer() error: %v", err)
	}
	if dialer == nil {
		t.Fatal("URLDialer() returned nil")
	}

	conn, err := dialer()
	if err != nil {
		t.Fatalf("dialer() error: %v", err)
	}
	if conn != c1 {
		t.Error("dialer() returned unexpected conn")
	}
}

func TestNetworkRegistry_StopAll(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	m1 := newMockNetwork("a")
	m2 := newMockNetwork("b")
	nr.Add(m1)
	nr.Add(m2)

	nr.StopAll()

	if !m1.isStopped() {
		t.Error("network 'a' should be stopped")
	}
	if !m2.isStopped() {
		t.Error("network 'b' should be stopped")
	}
}

func TestNetworkRegistry_Names(t *testing.T) {
	nr := newNetworkRegistryBare(nil)
	nr.Add(newMockNetwork("alpha"))
	nr.Add(newMockNetwork("beta"))

	names := nr.Names()
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

func TestNewNetworkRegistry_ContainsTCP(t *testing.T) {
	nr := NewNetworkRegistry(nil)
	for _, name := range []string{"tcp", "tcp4", "tcp6"} {
		if _, err := nr.Find(name); err != nil {
			t.Errorf("NewNetworkRegistry(nil) missing %q: %v", name, err)
		}
	}
}
