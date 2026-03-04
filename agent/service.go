package agent

import (
	"github.com/net-agent/remotework/agent/vservice"
)

// 类型别名 — 保持向后兼容，外部消费者（api/、cmd/）无需修改

type Service = vservice.Service
type ServiceController = vservice.ServiceController
type ServiceState = vservice.ServiceState
type ServiceStatus = vservice.ServiceStatus
type ServiceRegistry = vservice.ServiceRegistry
type ServiceSupervisor = vservice.ServiceSupervisor
type PortproxyController = vservice.PortproxyController
type Socks5Controller = vservice.Socks5Controller

const (
	StatusUninit  = vservice.StatusUninit
	StatusInit    = vservice.StatusInit
	StatusRunning = vservice.StatusRunning
	StatusStopped = vservice.StatusStopped
	StatusFailed  = vservice.StatusFailed
	StatusPending = vservice.StatusPending
)

var NewServiceRegistry = vservice.NewServiceRegistry
var NewServiceSupervisor = vservice.NewServiceSupervisor
