package api

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

// statsAnchorInterval stats 类事件的全量锚点间隔：增量/无变化跳过语义下，
// 可丢弃帧丢失后由锚点帧兜底收敛
const statsAnchorInterval = 30 * time.Second

// svcSnapshot 记录单个 service 的上次状态快照
type svcSnapshot struct {
	status  string
	actives int32
	dones   int32
	lastErr string
}

// StatePoller 后台轮询数据流变化，通过差异检测广播事件。
// 双节拍设计：
//   - 快节拍 statsInterval（秒级）：流快照采样、stream.stats 增量推送与
//     stream.update 开/关差分——字节数/速率等高频数据走这里；
//   - 慢节拍 interval：hub 运行状态、service 状态差分（v1 兼容）、
//     service.stats / network.stats 计数器快照。
//
// stats 类事件（时间无关 DTO，逐字段 == 比对）无变化不发帧；
// 每类事件至少每 statsAnchorInterval 发一帧全量锚点，兜住可丢弃帧的丢失。
// 跳过发帧不消耗主题 seq，客户端不会看到 seq 空洞。
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

	// 上次快照（stream.update 差分与 service.status v1 差分用）
	lastStreams   map[string]map[string]bool // network -> "localAddr->remoteAddr" set
	lastRunning   bool
	lastSvcStates map[string]svcSnapshot

	// stats 增量推送基线
	lastStreamStats  map[string]map[int32]StreamStateDTO // network -> index -> 上次已广播的 DTO
	streamAnchorAt   map[string]time.Time                // network -> 上次全量锚点帧时刻（零值=从未发，首帧必为全量）
	lastNetworkStats []NetworkStateDTO                   // 按 name 排序
	networkAnchorAt  time.Time
	lastServiceStats []ServiceStateDTO // 按 name 排序
	serviceAnchorAt  time.Time
}

func NewStatePoller(backend Backend, wsHub *WSHub, meter *streamMeter, interval, statsInterval time.Duration) *StatePoller {
	if meter == nil {
		meter = newStreamMeter()
	}
	return &StatePoller{
		backend:         backend,
		wsHub:           wsHub,
		meter:           meter,
		interval:        interval,
		statsInterval:   statsInterval,
		stopCh:          make(chan struct{}),
		lastStreams:     make(map[string]map[string]bool),
		lastRunning:     true,
		lastSvcStates:   make(map[string]svcSnapshot),
		lastStreamStats: make(map[string]map[int32]StreamStateDTO),
		streamAnchorAt:  make(map[string]time.Time),
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

	// 每 tick 计数器快照（计数/速率类高频数据，允许丢旧补新；无变化不发，锚点兜底）
	p.pollServiceStats()
	p.pollNetworkStats()
}

// pollNetworkStats 慢节拍：全量网络计数器快照（network.stats）。
// 无变化且距上次发送未满 statsAnchorInterval 时跳过
func (p *StatePoller) pollNetworkStats() {
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
	slices.SortFunc(networkDTOs, func(a, b NetworkStateDTO) int { return strings.Compare(a.Name, b.Name) })

	now := time.Now()
	if slices.Equal(p.lastNetworkStats, networkDTOs) && now.Sub(p.networkAnchorAt) < statsAnchorInterval {
		return
	}
	p.lastNetworkStats = networkDTOs
	p.networkAnchorAt = now
	p.wsHub.BroadcastDroppable("network.stats", map[string]interface{}{
		"networks": networkDTOs,
	})
}

// pollStreamStats 快节拍：拉取各网络流快照，增量广播 stream.stats
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
	seen := make(map[string]bool, len(streamStates))
	for _, ds := range streamStates {
		if ds == nil {
			continue
		}
		seen[ds.Network] = true

		// 活跃流采样实时速率（每 tick 必做：速率依赖相邻两次采样，与是否发帧无关）
		rates := p.meter.sample(ds.Network, ds.Actives, now)

		// 构建当前活跃流的 key 集合与完整 dto（含速率与归属服务）
		currentKeys := make(map[string]bool)
		currentDTOs := make([]StreamStateDTO, 0, len(ds.Actives))
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

		p.pushStreamStats(ds.Network, currentDTOs, now)

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

	// 网络下线：清理增量基线，防泄漏并避免重连后误比对
	for name := range p.lastStreamStats {
		if !seen[name] {
			delete(p.lastStreamStats, name)
			delete(p.streamAnchorAt, name)
		}
	}
}

// pushStreamStats 按增量语义广播一个网络的流状态：
// 仅自上次广播以来 DTO 有变化的流进入 streams（upsert 语义，客户端按 (network, index) 合并；
// 流的移除不走 stats，由可靠事件 stream.closed 送达）。
// 距上次全量帧满 statsAnchorInterval 时改发全量锚点帧（full=true，客户端整体替换
// 该网络的活跃流集合），兜住可丢弃帧的丢失。无变化且锚点未到时不发帧，
// 不消耗主题 seq，客户端不会看到 seq 空洞。
func (p *StatePoller) pushStreamStats(network string, dtos []StreamStateDTO, now time.Time) {
	prev := p.lastStreamStats[network]
	curr := make(map[int32]StreamStateDTO, len(dtos))
	var changed []StreamStateDTO
	for _, dto := range dtos {
		curr[dto.Index] = dto
		if prev[dto.Index] != dto {
			changed = append(changed, dto)
		}
	}
	p.lastStreamStats[network] = curr

	if now.Sub(p.streamAnchorAt[network]) >= statsAnchorInterval {
		full := dtos
		if full == nil {
			full = []StreamStateDTO{}
		}
		p.wsHub.BroadcastDroppable("stream.stats", map[string]interface{}{
			"network": network,
			"full":    true,
			"streams": full,
		})
		p.streamAnchorAt[network] = now
		return
	}
	if len(changed) > 0 {
		p.wsHub.BroadcastDroppable("stream.stats", map[string]interface{}{
			"network": network,
			"streams": changed,
		})
	}
}

func streamKeyFromState(st *stream.State) string {
	return fmt.Sprintf("%s:%s->%s:%s", st.LocalDomain, st.LocalAddr.String(), st.RemoteDomain, st.RemoteAddr.String())
}

// pollServiceStats 慢节拍：全量服务计数器快照（service.stats）。
// 无变化且距上次发送未满 statsAnchorInterval 时跳过
func (p *StatePoller) pollServiceStats() {
	states, err := p.backend.GetAllServiceState()
	if err != nil {
		return
	}
	dtos := make([]ServiceStateDTO, 0, len(states))
	for _, st := range states {
		dtos = append(dtos, toServiceDTO(st))
	}
	slices.SortFunc(dtos, func(a, b ServiceStateDTO) int { return strings.Compare(a.Name, b.Name) })

	now := time.Now()
	if slices.Equal(p.lastServiceStats, dtos) && now.Sub(p.serviceAnchorAt) < statsAnchorInterval {
		return
	}
	p.lastServiceStats = dtos
	p.serviceAnchorAt = now
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
