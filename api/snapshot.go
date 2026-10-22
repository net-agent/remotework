package api

import (
	"sync"
	"time"

	"github.com/net-agent/flex/v3/stream"
)

// 流生命周期事件合并窗口：窗口内的事件按网络合并为批量广播，
// 避免高并发开流场景逐条推送（一条事件携带数组）
const streamEventBatchWindow = 50 * time.Millisecond

// 支持订阅的主题
const (
	TopicHub     = "hub"
	TopicNetwork = "network"
	TopicService = "service"
	TopicStream  = "stream"
)

// snapshotOf 生成指定主题的快照事件（WSHub.sendSnapshot 的回调）；ok=false 表示主题不存在
func (s *Server) snapshotOf(topic string) (eventType string, data interface{}, ok bool) {
	switch topic {
	case TopicHub:
		return "hub.snapshot", s.hubSnapshot(), true
	case TopicNetwork:
		return "network.snapshot", s.networkSnapshot(), true
	case TopicService:
		return "service.snapshot", s.serviceSnapshot(), true
	case TopicStream:
		return "stream.snapshot", s.streamSnapshot(), true
	}
	return "", nil, false
}

// hubSnapshotData hub 主题快照载荷：statusDTO 全量 + startedAtMs。
// mode/configError 在进程生命周期内不变，version/commit/protocol/configPath 同为不变量，
// 客户端以快照为准即可，无需 hub.mode 类变更事件。
type hubSnapshotData struct {
	Running     bool   `json:"running"`
	Mode        string `json:"mode"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Protocol    int    `json:"protocol"`
	StartedAtMs int64  `json:"startedAtMs"` // API 服务器启动时刻；uptime 由客户端自行换算
	ConfigPath  string `json:"configPath"`
	ConfigError string `json:"configError,omitempty"`
}

func (s *Server) hubSnapshot() hubSnapshotData {
	return hubSnapshotData{
		Running:     s.backend.IsRunning(),
		Mode:        s.meta.Mode,
		Version:     s.meta.Version,
		Commit:      s.meta.Commit,
		Protocol:    s.meta.Protocol,
		StartedAtMs: s.startedAt.UnixMilli(),
		ConfigPath:  s.meta.ConfigPath,
		ConfigError: redactText(s.meta.ConfigError),
	}
}

// networkSnapshot 全部网络状态（与 REST /networks 一致，过滤内置 tcp4/tcp6）
func (s *Server) networkSnapshot() []NetworkStateDTO {
	reports, err := s.backend.GetAllNetworkState()
	if err != nil {
		return []NetworkStateDTO{}
	}
	dtos := make([]NetworkStateDTO, 0, len(reports))
	for _, r := range reports {
		if r.Name == "tcp4" || r.Name == "tcp6" {
			continue
		}
		dtos = append(dtos, toNetworkDTO(r))
	}
	return dtos
}

// serviceSnapshot 全部服务状态
func (s *Server) serviceSnapshot() []ServiceStateDTO {
	states, err := s.backend.GetAllServiceState()
	if err != nil {
		return []ServiceStateDTO{}
	}
	dtos := make([]ServiceStateDTO, 0, len(states))
	for _, st := range states {
		dtos = append(dtos, toServiceDTO(st))
	}
	return dtos
}

// streamSnapshot 各网络活跃流的扁平数组（已关闭流历史不进推送通道，留给 REST /streams）。
// 活跃流附带实时速率与归属服务标注（DTO 填充规则与 stream.stats 一致；本事件始终为全量基线）。
func (s *Server) streamSnapshot() []StreamStateDTO {
	reports, err := s.backend.GetAllNetworkState()
	if err != nil {
		return []StreamStateDTO{}
	}
	var networks []string
	for _, r := range reports {
		networks = append(networks, r.Name)
	}

	now := time.Now()
	dtos := []StreamStateDTO{}
	for _, ds := range s.backend.GetDataStreamState(0, networks...) {
		if ds == nil {
			continue
		}
		indexes := make([]int64, 0, len(ds.Actives))
		for _, st := range ds.Actives {
			if st != nil {
				indexes = append(indexes, int64(st.Index))
			}
		}
		annotations := annotateStreams(s.backend, ds.Network, indexes)

		rates := s.meter.sample(ds.Network, ds.Actives, now)
		for _, st := range ds.Actives {
			if st == nil {
				continue
			}
			dto := toStreamDTO(ds.Network, st)
			if rate, ok := rates[st.Index]; ok {
				dto.ReadBps = rate[0]
				dto.WriteBps = rate[1]
			}
			if ann, ok := annotations[int64(st.Index)]; ok {
				dto.ServiceName = ann.ServiceName
				dto.PipeID = ann.PipeID
			}
			s.resolver.enrich(ds.Network, st, &dto)
			dtos = append(dtos, dto)
		}
	}
	return dtos
}

// streamEventBatcher 把高频流生命周期事件按短暂窗口合并，flush 时按网络分组批量广播。
// 归属服务在 flush 时（而非事件发生时）标注：窗口延迟可避开服务归属注册的时序竞争。
type streamEventBatcher struct {
	mu     sync.Mutex
	opened map[string][]*stream.State // network -> states
	closed map[string][]*stream.State
}

func newStreamEventBatcher() *streamEventBatcher {
	return &streamEventBatcher{
		opened: make(map[string][]*stream.State),
		closed: make(map[string][]*stream.State),
	}
}

func (b *streamEventBatcher) add(evt StreamLifecycleEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if evt.Closed {
		b.closed[evt.Network] = append(b.closed[evt.Network], evt.State)
	} else {
		b.opened[evt.Network] = append(b.opened[evt.Network], evt.State)
	}
}

// flush 把累积的事件广播为空数组安全的分组事件；backend 用于给打开的流标注归属服务
// 与 pipe 配对（已关闭流的归属大概率已注销，与 poller 差分版 stream.update 一致保持空值）；
// resolver 给 inbound loopback 流标注客户端进程归属（含已关闭流，命中解析记忆）
func (b *streamEventBatcher) flush(hub *WSHub, backend Backend, resolver *clientProcResolver) {
	b.mu.Lock()
	opened, closed := b.opened, b.closed
	b.opened = make(map[string][]*stream.State)
	b.closed = make(map[string][]*stream.State)
	b.mu.Unlock()

	for network, states := range opened {
		indexes := make([]int64, 0, len(states))
		for _, st := range states {
			if st != nil {
				indexes = append(indexes, int64(st.Index))
			}
		}
		annotations := annotateStreams(backend, network, indexes)

		dtos := make([]StreamStateDTO, 0, len(states))
		for _, st := range states {
			if st == nil {
				continue
			}
			dto := toStreamDTO(network, st)
			if ann, ok := annotations[int64(st.Index)]; ok {
				dto.ServiceName = ann.ServiceName
				dto.PipeID = ann.PipeID
			}
			resolver.enrich(network, st, &dto)
			dtos = append(dtos, dto)
		}
		hub.Broadcast("stream.opened", map[string]interface{}{
			"network": network,
			"streams": dtos,
		})
	}
	for network, states := range closed {
		dtos := make([]StreamStateDTO, 0, len(states))
		for _, st := range states {
			if st == nil {
				continue
			}
			dto := toStreamDTO(network, st)
			resolver.enrich(network, st, &dto)
			dtos = append(dtos, dto)
		}
		hub.Broadcast("stream.closed", map[string]interface{}{
			"network": network,
			"streams": dtos,
		})
	}
}
