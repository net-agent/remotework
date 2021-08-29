package main

import (
	"fmt"
	"log"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/service"
)

func initServices(hub *agent.NetHub, cfg *agent.Config) {
	log.Println("startup services:")

	hub.AddServices(createTrusts(hub, cfg.Agents)...)
	hub.AddServices(createPortproxys(hub, cfg.Portproxy)...)
	hub.AddServices(createSocks5s(hub, cfg.Socks5)...)
	hub.AddServices(createQuickvisits(hub, cfg.Visit)...)
	hub.AddServices(createRDPs(hub, cfg.RDP)...)

	hub.StartServices()
}

func createTrusts(hub *agent.NetHub, agents []agent.AgentInfo) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range agents {
		if info.QuickTrust.Enable {
			svc := service.NewQuickTrust(
				hub,
				info.Network,
				info.QuickTrust.WhiteList,
				fmt.Sprintf("trust-%v", info.Network),
			)
			svcs = append(svcs, svc)
		}
	}
	return svcs
}

func createPortproxys(hub *agent.NetHub, pps []agent.PortproxyInfo) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range pps {
		svc := service.NewPortproxy(hub,
			info.ListenURL,
			info.TargetURL,
			info.LogName,
		)
		svcs = append(svcs, svc)
	}
	return svcs
}

func createQuickvisits(hub *agent.NetHub, visits []agent.QuickVisitInfo) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range visits {
		svc := service.NewQuickVisit(
			hub,
			info.ListenURL,
			info.TargetURL,
			info.LogName,
		)
		svcs = append(svcs, svc)
	}
	return svcs
}

func createSocks5s(hub *agent.NetHub, ss []agent.Socks5Info) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range ss {
		svc := service.NewSocks5(hub,
			info.ListenURL,
			info.Username,
			info.Password,
			info.LogName,
		)
		svcs = append(svcs, svc)
	}
	return svcs
}

func createRDPs(hub *agent.NetHub, rdps []agent.RDPInfo) []agent.Service {
	svcs := []agent.Service{}
	for _, info := range rdps {
		svc := service.NewRDP(hub, info.ListenURL, info.LogName)
		svcs = append(svcs, svc)
	}
	return svcs
}
