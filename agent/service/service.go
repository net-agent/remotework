package service

import (
	"fmt"

	"github.com/net-agent/remotework/agent"
)

type Service interface {
	Run() error
	Close() error
	Info() string
}

func NewService(mnet *agent.MixNet, info agent.ServiceInfo) Service {
	switch info.Type {
	case "socks5": // socks5 server
		return NewSocks5(mnet, info)
	case "portproxy": // port proxy server
		return NewPortproxy(mnet, info)
	case "rdp": // remote desktop protocol
		info.Param["target"] = fmt.Sprintf("tcp://localhost:%v", rdpPortNumber())
		return NewPortproxy(mnet, info)
	case "rce": // remote code execution
		return nil

	// 快速信赖服务
	case "quick-trust":
		return NewQuickTrust(mnet, info)
	// 快速访问服务
	case "quick-visit":
		return NewQuickVisit(mnet, info)
	}
	return nil
}
