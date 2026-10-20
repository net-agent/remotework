package vservice

import (
	"testing"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

// TestLinkConns_Paired 两端连接均可提取编号时建立配对：
// 两端 MatchPipe 命中相同 pipeID，且随各自连接的注销独立清除
func TestLinkConns_Paired(t *testing.T) {
	p1, _ := pipePair(t)
	p2, _ := pipePair(t)
	a := &mockConnIDConn{Conn: p1, id: 7}
	b := &mockStreamConn{Conn: p2, state: &stream.State{Index: 42}}
	s := &ServiceState{Name: "svc"}

	pipeID := s.LinkConns("tcp", a, "link1", b)
	if pipeID == "" {
		t.Fatal("LinkConns should succeed when both ends resolvable")
	}
	if pipeID != "svc#1" {
		t.Errorf("pipeID = %q, want %q", pipeID, "svc#1")
	}
	if got := s.MatchPipe("tcp", 7); got != pipeID {
		t.Errorf("MatchPipe(tcp,7) = %q, want %q", got, pipeID)
	}
	if got := s.MatchPipe("link1", 42); got != pipeID {
		t.Errorf("MatchPipe(link1,42) = %q, want %q", got, pipeID)
	}
	if got := s.MatchPipe("tcp", 8); got != "" {
		t.Errorf("MatchPipe on wrong index = %q, want empty", got)
	}

	// 序号递增：第二次配对新 id
	if second := s.LinkConns("tcp", a, "link1", b); second != "svc#2" {
		t.Errorf("second pipeID = %q, want %q", second, "svc#2")
	}

	// 一端注销只清自己的映射，另一端保留（与归属登记生命周期一致）
	s.untrackConn(connRef{Network: "tcp", Index: 7})
	if got := s.MatchPipe("tcp", 7); got != "" {
		t.Errorf("MatchPipe(tcp,7) after untrack = %q, want empty", got)
	}
	if got := s.MatchPipe("link1", 42); got != "svc#2" {
		t.Errorf("MatchPipe(link1,42) should survive peer untrack, got %q", got)
	}
}

// TestLinkConns_Unresolvable 任一端无法提取编号时不建立关联：
// 返回空且另一端也不留映射（避免单边 pipeId 误导观测）
func TestLinkConns_Unresolvable(t *testing.T) {
	p1, _ := pipePair(t)
	p2, _ := pipePair(t)
	a := &mockConnIDConn{Conn: p1, id: 7}
	s := &ServiceState{Name: "svc"}

	if got := s.LinkConns("tcp", a, "tcp", p2); got != "" {
		t.Fatalf("LinkConns with plain conn = %q, want empty", got)
	}
	if got := s.MatchPipe("tcp", 7); got != "" {
		t.Errorf("resolvable side should not be paired when peer fails, got %q", got)
	}
	if got := s.LinkConns("", a, "tcp", a); got != "" {
		t.Errorf("empty network should yield empty pipeID, got %q", got)
	}
	if got := s.LinkConns("tcp", nil, "tcp", a); got != "" {
		t.Errorf("nil conn should yield empty pipeID, got %q", got)
	}
}

// TestLinkConns_ThroughTrackWrapper 模拟调用点的真实形态：
// 连接先被 TrackConn 包装，LinkConns 仍能穿透包装提取两端编号
func TestLinkConns_ThroughTrackWrapper(t *testing.T) {
	p1, _ := pipePair(t)
	p2, _ := pipePair(t)
	s := &ServiceState{Name: "svc"}

	wrappedA := s.TrackConn("tcp", &mockConnIDConn{Conn: p1, id: 7})
	wrappedB := s.TrackConn("link1", &mockStreamConn{Conn: p2, state: &stream.State{Index: 42}})

	pipeID := s.LinkConns("tcp", wrappedA, "link1", wrappedB)
	if pipeID == "" {
		t.Fatal("LinkConns should resolve through TrackConn wrappers")
	}
	if s.MatchPipe("tcp", 7) != pipeID || s.MatchPipe("link1", 42) != pipeID {
		t.Error("both ends should be paired through wrappers")
	}

	// 包装连接 Close 触发注销，配对同步清除
	wrappedA.Close()
	wrappedB.Close()
	if s.MatchPipe("tcp", 7) != "" || s.MatchPipe("link1", 42) != "" {
		t.Error("pipe mapping should be removed after conns closed")
	}
}

// TestMatchPipe_Empty 空状态服务不匹配任何配对
func TestMatchPipe_Empty(t *testing.T) {
	s := &ServiceState{}
	if got := s.MatchPipe("tcp", 1); got != "" {
		t.Errorf("empty state should not match any pipe, got %q", got)
	}
}

// waitPipe 轮询等待指定连接的配对 pipeID 出现。
// 配对登记（LinkConns）发生在归属登记（TrackConn）之后，MatchConn 命中
// 不代表配对已就绪，涉及配对的断言必须经此轮询
func waitPipe(svc *Service, network string, index int64, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pipeID := svc.MatchPipe(network, index); pipeID != "" {
			return pipeID
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ""
}
