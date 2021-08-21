package service

import (
	"fmt"
	"log"
	"sync"

	"github.com/net-agent/remotework/agent"
)

type Service interface {
	// Start 开启服务
	// 做好开启服务需要的准备，然后启动协程运行任务，同时返回准备过程中发生的错误
	Start(wg *sync.WaitGroup) error
	Close() error
	Info() string
}

func NewService(hub *agent.NetHub, info agent.ServiceInfo) Service {
	switch info.Type {
	case "socks5": // socks5 server
		return NewSocks5(hub, info)
	case "portproxy": // port proxy server
		return NewPortproxy(hub, info)
	case "rdp": // remote desktop protocol
		info.Param["target"] = fmt.Sprintf("tcp://localhost:%v", rdpPortNumber())
		info.Param["type"] = "rdp" // rewrite type
		return NewPortproxy(hub, info)
	case "rce": // remote code execution
		return nil

	// 快速信赖服务，转移至agent中设置
	case "quick-trust":
		return nil
	// 快速访问服务
	case "quick-visit":
		return NewQuickVisit(hub, info)
	}
	return nil
}

func runsvc(svcName string, wg *sync.WaitGroup, runner func()) {
	if wg != nil {
		wg.Add(1)
	}
	log.Printf("[runsvc] service start. name=%v\n", svcName)
	go func() {
		runner()
		if wg != nil {
			wg.Done()
		}
		log.Printf("[runsvc] service stopped. name=%v\n", svcName)
	}()
}
