package main

import (
	"github.com/net-agent/remotework/agent"
)

func initServices(hub *agent.Hub, cfg *agent.Config) {
	hub.AddServices(createPortproxys(hub, cfg.Portproxy)...)
	hub.AddServices(createSocks5s(hub, cfg.Socks5)...)
	hub.AddServices(createRDPs(hub, cfg.RDP)...)
}

func createPortproxys(hub *agent.Hub, pps []agent.PortproxyInfo) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range pps {
		svc := agent.NewPortproxy(hub,
			info.ListenURL,
			info.TargetURL,
			info.LogName,
		)
		svcs = append(svcs, svc)
	}
	return svcs
}

func createSocks5s(hub *agent.Hub, ss []agent.Socks5Info) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range ss {
		svc := agent.NewSocks5(hub,
			info.ListenURL,
			info.Username,
			info.Password,
			info.LogName,
		)
		svcs = append(svcs, svc)
	}
	return svcs
}

func createRDPs(hub *agent.Hub, rdps []agent.RDPInfo) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range rdps {
		svc := agent.NewRDP(hub, info.ListenURL, info.LogName)
		svcs = append(svcs, svc)
	}
	return svcs
}
