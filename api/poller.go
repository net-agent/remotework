package api

import (
	"fmt"
	"time"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/agent"
)

// svcSnapshot 记录单个 service 的上次状态快照
type svcSnapshot struct {
	status  string
	actives int32
	dones   int32
	lastErr string
}

// StatePoller 后台轮询数据流变化，通过差异检测广播事件
// 注意：network 状态已通过 Session 回调链实时推送，不再轮询
type StatePoller struct {
	hub      *agent.Hub
	wsHub    *WSHub
	interval time.Duration
	stopCh   chan struct{}

	// 上次快照
	lastStreams    map[string]map[string]bool // network -> "localAddr->remoteAddr" set
	lastRunning   bool
	lastSvcStates map[string]svcSnapshot
}

func NewStatePoller(hub *agent.Hub, wsHub *WSHub, interval time.Duration) *StatePoller {
	return &StatePoller{
		hub:           hub,
		wsHub:         wsHub,
		interval:      interval,
		stopCh:        make(chan struct{}),
		lastStreams:    make(map[string]map[string]bool),
		lastRunning:   true,
		lastSvcStates: make(map[string]svcSnapshot),
	}
}

func (p *StatePoller) Start() {
	go p.run()
}

func (p *StatePoller) Stop() {
	close(p.stopCh)
}

func (p *StatePoller) run() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

func (p *StatePoller) poll() {
	// 检查 hub 运行状态
	running := p.hub.IsRunning()
	if p.lastRunning && !running {
		p.wsHub.Broadcast("hub.stopped", map[string]interface{}{})
	}
	p.lastRunning = running

	// 轮询 service 状态变化
	p.pollServices()

	// 获取网络名称列表用于 stream 轮询
	reports, err := p.hub.GetAllNetworkState()
	if err != nil {
		return
	}

	var networks []string
	for _, r := range reports {
		networks = append(networks, r.Name)
	}

	streamStates := p.hub.GetDataStreamState(100, networks...)

	for _, ds := range streamStates {
		if ds == nil {
			continue
		}

		// 构建当前活跃流的 key 集合
		currentKeys := make(map[string]bool)
		var currentDTOs []StreamStateDTO
		for _, st := range ds.Actives {
			key := streamKeyFromState(st)
			currentKeys[key] = true
			currentDTOs = append(currentDTOs, toStreamDTO(ds.Network, st))
		}

		prevKeys := p.lastStreams[ds.Network]
		if prevKeys == nil {
			prevKeys = make(map[string]bool)
		}

		// 找出新增的流
		var opened []StreamStateDTO
		for i, dto := range currentDTOs {
			key := streamKeyFromState(ds.Actives[i])
			if !prevKeys[key] {
				opened = append(opened, dto)
			}
		}

		// 找出关闭的流（在上次快照中但不在当前活跃中）
		var closed []StreamStateDTO
		for _, st := range ds.Closeds {
			key := streamKeyFromState(st)
			if prevKeys[key] && !currentKeys[key] {
				closed = append(closed, toStreamDTO(ds.Network, st))
			}
		}

		if len(opened) > 0 || len(closed) > 0 {
			p.wsHub.Broadcast("stream.update", map[string]interface{}{
				"network": ds.Network,
				"opened":  opened,
				"closed":  closed,
			})
		}

		p.lastStreams[ds.Network] = currentKeys
	}
}

func streamKeyFromState(st *stream.State) string {
	return fmt.Sprintf("%s:%s->%s:%s", st.LocalDomain, st.LocalAddr.String(), st.RemoteDomain, st.RemoteAddr.String())
}

// pollServices 检测 service 状态变化并广播事件
func (p *StatePoller) pollServices() {
	states, err := p.hub.GetAllServiceState()
	if err != nil {
		return
	}

	for _, s := range states {
		prev, exists := p.lastSvcStates[s.Name]
		curr := svcSnapshot{
			status:  s.StatusString(),
			actives: s.GetActiveCount(),
			dones:   s.GetDoneCount(),
			lastErr: s.LastErr,
		}

		if !exists || prev != curr {
			oldStatus := prev.status
			if !exists {
				oldStatus = ""
			}
			dto := toServiceDTO(s)
			p.wsHub.Broadcast("service.status", map[string]interface{}{
				"name":      s.Name,
				"oldStatus": oldStatus,
				"newStatus": curr.status,
				"service":   dto,
			})
			p.lastSvcStates[s.Name] = curr
		}
	}
}
