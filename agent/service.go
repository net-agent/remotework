package agent

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/net-agent/remotework/utils"
)

type Service struct {
	ServiceState
	controller ServiceController
}

type ServiceController interface {
	Init() error
	Start() error
	Close() error
	Update() error // 依赖的netnode重连后，能够更新runner
}

type ServiceState struct {
	Type      string
	Name      string
	ListenURL string
	TargetURL string
	Username  string
	Password  string

	State   string
	ID      int32
	Actives int32
	Dones   int32
}

func (s *ServiceState) AddActiveCount(n int32) { atomic.AddInt32(&s.Actives, 1) }
func (s *ServiceState) AddDoneCount(n int32) {
	atomic.AddInt32(&s.Actives, -1)
	atomic.AddInt32(&s.Dones, 1)
}
func (s *ServiceState) GetActiveCount() int32  { return s.Actives }
func (s *ServiceState) GetDoneCount() int32    { return s.Dones }
func (s *ServiceState) IsDepend(n string) bool { return strings.HasPrefix(s.ListenURL, n) }

//
// service constructors
//

func NewPortproxyService(hub *Hub, info PortproxyInfo) *Service {
	svc := &Service{}

	svc.Type = "portproxy"
	svc.Name = utils.FirstString(info.LogName, "portproxy")
	svc.ListenURL = info.ListenURL
	svc.TargetURL = info.TargetURL
	svc.controller = NewPortproxyController(hub, &svc.ServiceState)

	return svc
}

func NewRDPService(hub *Hub, info RDPInfo) *Service {
	svc := &Service{}

	svc.Type = "rdpserver"
	svc.Name = utils.FirstString(info.LogName, "rdp")
	svc.ListenURL = info.ListenURL
	svc.TargetURL = fmt.Sprintf("tcp://localhost:%v", utils.GetRDPPort())
	svc.controller = NewPortproxyController(hub, &svc.ServiceState)

	return svc
}

func NewSocks5Service(hub *Hub, info Socks5Info) *Service {
	svc := &Service{}

	svc.Type = "socks5"
	svc.Name = utils.FirstString(info.LogName, "socks5")
	svc.ListenURL = info.ListenURL
	svc.Username = info.Username
	svc.Password = info.Password
	svc.controller = NewSocks5Controller(hub, &svc.ServiceState)

	return svc
}
