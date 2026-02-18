package api

import (
	"time"

	"github.com/net-agent/flex/v2/stream"
	"github.com/net-agent/remotework/agent"
)

type ServiceStateDTO struct {
	ID        int32  `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	ListenURL string `json:"listenURL"`
	TargetURL string `json:"targetURL"`
	Actives   int32  `json:"actives"`
	Dones     int32  `json:"dones"`
}

type NetworkStateDTO struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Domain   string `json:"domain"`
	State    string `json:"state"`
	LastErr  string `json:"lastErr,omitempty"`
	AliveMs  int64  `json:"aliveMs"`
	Listens  int32  `json:"listens"`
	Dials    int32  `json:"dials"`
}

type StreamStateDTO struct {
	Network      string `json:"network"`
	LocalDomain  string `json:"localDomain"`
	LocalAddr    string `json:"localAddr"`
	RemoteDomain string `json:"remoteDomain"`
	RemoteAddr   string `json:"remoteAddr"`
	ReadBytes    int64  `json:"readBytes"`
	WriteBytes   int64  `json:"writeBytes"`
	AliveMs      int64  `json:"aliveMs"`
	IsClosed     bool   `json:"isClosed"`
}

type PingResultDTO struct {
	Network      string   `json:"network"`
	Domain       string   `json:"domain"`
	Result       string   `json:"result"`
	UsedServices []string `json:"usedServices"`
}

func toServiceDTO(s agent.ServiceState) ServiceStateDTO {
	return ServiceStateDTO{
		ID:        s.ID,
		Type:      s.Type,
		Name:      s.Name,
		Status:    s.StatusString(),
		ListenURL: s.ListenURL,
		TargetURL: s.TargetURL,
		Actives:   s.GetActiveCount(),
		Dones:     s.GetDoneCount(),
	}
}

func toNetworkDTO(r agent.NetworkReport) NetworkStateDTO {
	return NetworkStateDTO{
		Name:     r.Name,
		Protocol: r.Protocol,
		Address:  r.Address,
		Domain:   r.Domain,
		State:    r.State,
		LastErr:  r.LastErr,
		AliveMs:  r.Alive.Milliseconds(),
		Listens:  r.Listens,
		Dials:    r.Dials,
	}
}

func toStreamDTO(network string, s *stream.State) StreamStateDTO {
	aliveMs := time.Since(s.Created).Milliseconds()
	if s.IsClosed {
		aliveMs = s.Closed.Sub(s.Created).Milliseconds()
	}
	return StreamStateDTO{
		Network:      network,
		LocalDomain:  s.LocalDomain,
		LocalAddr:    s.LocalAddr.String(),
		RemoteDomain: s.RemoteDomain,
		RemoteAddr:   s.RemoteAddr.String(),
		ReadBytes:    s.ConnReadSize,
		WriteBytes:   s.ConnWriteSize,
		AliveMs:      aliveMs,
		IsClosed:     s.IsClosed,
	}
}

func toPingDTO(r *agent.PingReport) PingResultDTO {
	return PingResultDTO{
		Network:      r.Network,
		Domain:       r.Domain,
		Result:       r.PingResult,
		UsedServices: r.UsedServices,
	}
}
