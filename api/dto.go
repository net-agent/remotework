package api

import (
	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

type ServiceStateDTO struct {
	ID        int32  `json:"id"`
	TunnelID  string `json:"tunnelId,omitempty"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	LastErr   string `json:"lastErr,omitempty"`
	ListenURL string `json:"listenURL"`
	TargetURL string `json:"targetURL"`
	Actives   int32  `json:"actives"`
	Dones     int32  `json:"dones"`
}

type NetworkStateDTO struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Domain   string `json:"domain"`
	State    string `json:"state"`
	LastErr  string `json:"lastErr,omitempty"`
	// StartedAtMs 为时间无关的绝对锚点：存活时长由客户端按 now-startedAtMs 自行换算，
	// 避免每 tick 必变的派生字段破坏增量比对
	StartedAtMs int64 `json:"startedAtMs"` // 网络对象创建时刻（Unix 毫秒）
	Listens     int32 `json:"listens"`
	Dials       int32 `json:"dials"`
	Reconnects  int64 `json:"reconnects"`

	BytesRead    int64 `json:"bytesRead"`    // 节点累计读取字节数
	BytesWritten int64 `json:"bytesWritten"` // 节点累计写入字节数
	RTTMs        int64 `json:"rttMs"`        // 最近一次心跳 RTT（毫秒，0 表示未测量）
}

type StreamStateDTO struct {
	Network      string `json:"network"`
	Index        int32  `json:"index"`
	Direction    string `json:"direction"` // outbound / inbound / unknown
	LocalDomain  string `json:"localDomain"`
	LocalAddr    string `json:"localAddr"`
	RemoteDomain string `json:"remoteDomain"`
	RemoteAddr   string `json:"remoteAddr"`

	// 客户端进程归属：仅 inbound 且 remoteAddr 为 loopback 的流尝试解析（开流时解析一次并记忆，
	// 由 clientProcResolver 填充）；其余场景与解析失败（权限不足/进程已退出/非 TCP 传输）保持零值省略
	ClientPid         int32  `json:"clientPid,omitempty"`         // 客户端进程 PID
	ClientProcName    string `json:"clientProcName,omitempty"`    // 客户端进程名
	ClientProcStartMs int64  `json:"clientProcStartMs,omitempty"` // 客户端进程启动时刻（Unix 毫秒），供甄别 PID 复用

	ReadBytes  int64 `json:"readBytes"`
	WriteBytes int64 `json:"writeBytes"`
	// CreatedAtMs/ClosedAtMs 为时间无关的绝对锚点：存活时长由客户端按
	// now-createdAtMs（或 closedAtMs-createdAtMs）自行换算
	CreatedAtMs int64 `json:"createdAtMs"`          // 流发起时刻（Unix 毫秒）
	ClosedAtMs  int64 `json:"closedAtMs,omitempty"` // 流完全关闭时刻（Unix 毫秒），仅已关闭流携带
	IsClosed    bool  `json:"isClosed"`

	ReadClosed  bool   `json:"readClosed"`            // 读方向已关闭（半关闭状态）
	WriteClosed bool   `json:"writeClosed"`           // 写方向已关闭（半关闭状态）
	CloseReason string `json:"closeReason,omitempty"` // 关闭原因：local/remote/node/manual

	SentBufferCount int32 `json:"sentBufferCount"`
	RecvBufferCount int32 `json:"recvBufferCount"`
	RecvDataSize    int64 `json:"recvDataSize"`
	RecvAckTotal    int64 `json:"recvAckTotal"`
	SentAckTotal    int64 `json:"sentAckTotal"`

	ReadBps     float64 `json:"readBps"`               // 实时读流速（字节/秒），由调用方填充
	WriteBps    float64 `json:"writeBps"`              // 实时写流速（字节/秒），由调用方填充
	ServiceName string  `json:"serviceName,omitempty"` // 归属服务名，由调用方填充
	PipeID      string  `json:"pipeId,omitempty"`      // pipe 配对标识：tunnel 两端流携带相同值，由调用方填充
}

type PingResultDTO struct {
	Network      string   `json:"network"`
	Domain       string   `json:"domain"`
	Result       string   `json:"result"`
	UsedServices []string `json:"usedServices"`
}

// statusDTO GET /status 的响应体（sidecar 协议 §6.1）
type statusDTO struct {
	Running     bool   `json:"running"`
	Mode        string `json:"mode"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Protocol    int    `json:"protocol"`
	StartedAtMs int64  `json:"startedAtMs"` // API 服务器启动时刻（Unix 毫秒）；uptime 由客户端自行换算
	ConfigPath  string `json:"configPath"`
	ConfigError string `json:"configError,omitempty"`
}

type ListeningPortDTO struct {
	Port        uint16 `json:"port"`
	Protocol    string `json:"protocol"`
	Address     string `json:"address,omitempty"` // 监听地址（如 0.0.0.0 / :: / 127.0.0.1）
	PID         uint32 `json:"pid,omitempty"`
	ProcessName string `json:"processName,omitempty"`
	ProcStartMs int64  `json:"procStartMs,omitempty"` // 进程启动时刻（Unix 毫秒）；注意是进程启动时间，非开始监听时间
}

func toServiceDTO(s *vservice.ServiceState) ServiceStateDTO {
	return ServiceStateDTO{
		ID:        s.ID,
		TunnelID:  s.TunnelID,
		Type:      s.Type,
		Name:      s.Name,
		Status:    s.StatusString(),
		LastErr:   s.LastErr,
		ListenURL: s.ListenURL,
		TargetURL: s.TargetURL,
		Actives:   s.GetActiveCount(),
		Dones:     s.GetDoneCount(),
	}
}

func toNetworkDTO(r vnet.NetworkReport) NetworkStateDTO {
	return NetworkStateDTO{
		Name:         r.Name,
		Kind:         r.Kind,
		Protocol:     r.Protocol,
		Address:      r.Address,
		Domain:       r.Domain,
		State:        r.State,
		LastErr:      r.LastErr,
		StartedAtMs:  r.StartedAt.UnixMilli(),
		Listens:      r.Listens,
		Dials:        r.Dials,
		Reconnects:   r.Reconnects,
		BytesRead:    r.BytesRead,
		BytesWritten: r.BytesWritten,
		RTTMs:        r.RTT.Milliseconds(),
	}
}

// directionString 把 flex 流方向枚举映射为对外文案
func directionString(d stream.Direction) string {
	switch d {
	case stream.DirectionOutbound:
		return "outbound"
	case stream.DirectionInbound:
		return "inbound"
	default:
		return "unknown"
	}
}

// toStreamDTO 纯字段映射：flex 状态全量透传（时间无关，同一状态产出相等 DTO，
// 供增量比对直接 ==）；实时速率（ReadBps/WriteBps）与归属服务（ServiceName）由 handler/poller 二次填充
func toStreamDTO(network string, s *stream.State) StreamStateDTO {
	var closedAtMs int64
	if s.IsClosed {
		closedAtMs = s.Closed.UnixMilli()
	}
	return StreamStateDTO{
		Network:         network,
		Index:           s.Index,
		Direction:       directionString(s.Direction),
		LocalDomain:     s.LocalDomain,
		LocalAddr:       s.LocalAddr.String(),
		RemoteDomain:    s.RemoteDomain,
		RemoteAddr:      s.RemoteAddr.String(),
		ReadBytes:       s.BytesRead,
		WriteBytes:      s.BytesWritten,
		CreatedAtMs:     s.Created.UnixMilli(),
		ClosedAtMs:      closedAtMs,
		IsClosed:        s.IsClosed,
		ReadClosed:      s.ReadClosed,
		WriteClosed:     s.WriteClosed,
		CloseReason:     s.CloseReason,
		SentBufferCount: s.SentBufferCount,
		RecvBufferCount: s.RecvBufferCount,
		RecvDataSize:    s.RecvDataSize,
		RecvAckTotal:    s.RecvAckTotal,
		SentAckTotal:    s.SentAckTotal,
	}
}

func toPingDTO(r *vnet.PingReport) PingResultDTO {
	return PingResultDTO{
		Network:      r.Network,
		Domain:       r.Domain,
		Result:       r.PingResult,
		UsedServices: r.UsedServices,
	}
}
