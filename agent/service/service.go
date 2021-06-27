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

func NewService(info agent.ServiceInfo) Service {
	switch info.Type {
	case "socks5": // socks5 server
		return NewSocks5(info)
	case "portproxy": // port proxy server
		return NewPortproxy(info)
	case "rdp": // remote desktop protocol
		info.Param["target"] = fmt.Sprintf("tcp://localhost:%v", rdpPortNumber())
		return NewPortproxy(info)
	case "rce": // remote code execution
		return nil
	}
	return nil
}

func color(color int, info string) string { return fmt.Sprintf("\x1b[%dm%v\x1b[0m", color, info) }
func green(info string) string            { return color(32, info) }
func red(info string) string              { return color(31, info) }
func yellow(info string) string           { return color(33, info) }