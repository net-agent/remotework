package api

import (
	"fmt"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

// svcSnapshot 记录单个 service 的上次状态快照
type svcSnapshot struct {
	status  string
	actives int32
	dones   int32
	lastErr string
}

// StatePoller 后台轮询数据流变化，通过差异检测广播事件。
// 双节拍设计：
//   - 快节拍 statsInterval（秒级）：流快照采样、stream.stats 全量推送与
//     stream.update 开/关差分——字节数/速率等高频数据走这里；
//   - 慢节拍 interval：hub 运行状态、service 状态差分（v1 兼容）、
//     service.stats / network.stats 计数器快照。
//
// 注意：network/service 状态变化与流开/关（stream.opened/closed）已通过
// 事件订阅实时推送，轮询仅兜底差分、流速采样与计数器刷新。
type StatePoller struct {
	backend       Backend
	wsHub         *WSHub
	meter         *streamMeter // 流速率计，与 Server 共享
	interval      time.Duration
	statsInterval time.Duration
	stopCh        chan struct{}

	// 上次快照
	lastStreams   map[string]map[string]bool // network -> "localAddr->remoteAddr" set
	lastRunning   bool
	lastSvcStates map[string]svcSnapshot
}

func NewStatePoller(backend Backend, wsHub *WSHub, meter *streamMeter, interval, statsInterval time.Duration) *StatePoller {
	if meter == nil {
		meter = newStreamMeter()
	}
	return &StatePoller{
		backend:       backend,
		wsHub:         wsHub,
		meter:         meter,
		interval:      interval,
		statsInterval: statsInterval,
		stopCh:        make(chan struct{}),
		lastStreams:   make(map[string]map[string]bool),
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
	statsTicker := time.NewTicker(p.statsInterval)
	defer ticker.Stop()
	defer statsTicker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.poll()
		case <-statsTicker.C:
			p.pollStreamStats()
		}
	}
}

// poll 慢节拍：hub 运行状态、service 状态差分、service/network 计数器快照
func (p *StatePoller) poll() {
	// 检查后端运行状态
	running := p.backend.IsRunning()
	if p.lastRunning && !running {
		p.wsHub.Broadcast("hub.stopped", map[string]interface{}{})
	}
	p.lastRunning = running

	// 轮询 service 状态变化（v1 差分广播，保留兼容；v2 客户端以 service.stats 为准）
	p.pollServices()

	// 每 tick 全量计数器快照（计数/速率类高频数据，允许丢旧补新）
	p.pollServiceStats()

	// network.stats：每 tick 全量网络快照（含计数器、字节数、RTT）
	reports, err := p.backend.GetAllNetworkState()
	if err != nil {
		return
	}
	networkDTOs := make([]NetworkStateDTO, 0, len(reports))
	for _, r := range reports {
		if r.Name == "tcp4" || r.Name == "tcp6" {
			continue
		}
		networkDTOs = append(networkDTOs, toNetworkDTO(r))
	}
	p.wsHub.BroadcastDroppable("network.stats", map[string]interface{}{
		"networks": networkDTOs,
	})
}

// pollStreamStats 快节拍：拉取各网络流快照，广播 stream.stats 全量
// （含实时速率）并做开/关差分（stream.update，事件通道的兜底）
func (p *StatePoller) pollStreamStats() {
	// 获取网络名称列表用于 stream 轮询
	reports, err := p.backend.GetAllNetworkState()
	if err != nil {
		return
	}

	var networks []string
	for _, r := range reports {
		networks = append(networks, r.Name)
	}

	streamStates := p.backend.GetDataStreamState(100, networks...)

	now := time.Now()
	for _, ds := range streamStates {
		if ds == nil {
			continue
		}

		// 活跃流采样实时速率
		rates := p.meter.sample(ds.Network, ds.Actives, now)

		// 构建当前活跃流的 key 集合与完整 dto（含速率与归属服务）
		currentKeys := make(map[string]bool)
		var currentDTOs []StreamStateDTO
		for _, st := range ds.Actives {
			key := streamKeyFromState(st)
			currentKeys[key] = true
			dto := toStreamDTO(ds.Network, st)
			if rate, ok := rates[st.Index]; ok {
				dto.ReadBps = rate[0]
				dto.WriteBps = rate[1]
			}
			dto.ServiceName = p.backend.GetStreamService(ds.Network, int64(st.Index))
			dto.PipeID = p.backend.GetStreamPipe(ds.Network, int64(st.Index))
			currentDTOs = append(currentDTOs, dto)
		}

		// 周期快照推送：仅活跃流，每 tick 每网络广播一次（可丢弃类）
		statsDTOs := currentDTOs
		if statsDTOs == nil {
			statsDTOs = []StreamStateDTO{}
		}
		p.wsHub.BroadcastDroppable("stream.stats", map[string]interface{}{
			"network": ds.Network,
			"streams": statsDTOs,
		})

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

		// 找出关闭的流（在上次快照中但不在当前活跃中）；
		// 关闭流的归属服务大概率已注销，ServiceName 保持空值即可
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

// pollServiceStats 每 tick 广播全量服务计数器快照（service.stats）
func (p *StatePoller) pollServiceStats() {
	states, err := p.backend.GetAllServiceState()
	if err != nil {
		return
	}
	dtos := make([]ServiceStateDTO, 0, len(states))
	for _, st := range states {
		dtos = append(dtos, toServiceDTO(st))
	}
	p.wsHub.BroadcastDroppable("service.stats", map[string]interface{}{
		"services": dtos,
	})
}

// pollServices 检测 service 状态变化并广播事件
func (p *StatePoller) pollServices() {
	states, err := p.backend.GetAllServiceState()
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
