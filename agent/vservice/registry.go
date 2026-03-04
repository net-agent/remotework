package vservice

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/net-agent/remotework/utils"
)

// ServiceRegistry 管理服务的注册、查找和状态报告
type ServiceRegistry struct {
	log   *slog.Logger
	svcs  []*Service
	names map[string]*Service
	mut   sync.RWMutex
	id    int32
}

func NewServiceRegistry(log *slog.Logger) *ServiceRegistry {
	if log == nil {
		log = utils.NewModuleLogger("hub.svc")
	}
	return &ServiceRegistry{
		log:   log,
		names: make(map[string]*Service),
	}
}

func (sr *ServiceRegistry) Add(svc *Service) error {
	svc.ID = atomic.AddInt32(&sr.id, 1)

	sr.mut.Lock()
	defer sr.mut.Unlock()

	if _, found := sr.names[svc.Name]; found {
		return errors.New("duplicate service name")
	}

	svc.SetStatus(StatusUninit)
	sr.svcs = append(sr.svcs, svc)
	sr.names[svc.Name] = svc

	return nil
}

func (sr *ServiceRegistry) Find(name string) (*Service, error) {
	sr.mut.RLock()
	defer sr.mut.RUnlock()

	svc, found := sr.names[name]
	if !found {
		return nil, errors.New("service not found")
	}
	return svc, nil
}

func (sr *ServiceRegistry) Range(fn func(svc *Service)) {
	sr.mut.RLock()
	svcs := make([]*Service, len(sr.svcs))
	copy(svcs, sr.svcs)
	sr.mut.RUnlock()

	for _, svc := range svcs {
		fn(svc)
	}
}

// Names 返回所有已注册的服务名称
func (sr *ServiceRegistry) Names() []string {
	sr.mut.RLock()
	defer sr.mut.RUnlock()

	names := make([]string, 0, len(sr.names))
	for name := range sr.names {
		names = append(names, name)
	}
	return names
}

//
// 状态报告方法
//

func (sr *ServiceRegistry) GetAllState() ([]ServiceState, error) {
	sr.mut.RLock()
	defer sr.mut.RUnlock()

	if len(sr.svcs) <= 0 {
		return nil, errors.New("NO SERVICES")
	}

	var reports []ServiceState
	for _, svc := range sr.svcs {
		reports = append(reports, svc.ServiceState)
	}
	return reports, nil
}

func (sr *ServiceRegistry) GetAllStateString() string {
	reports, err := sr.GetAllState()
	if err != nil {
		return fmt.Sprintf("report service failed: %v\n", err)
	}

	buf := bytes.NewBufferString("report service:\n")
	utils.RenderAsciiTable(buf, reports,
		[]string{"index", "type", "name", "state", "listen", "target", "actives", "dones"},
		func(d interface{}, index int) []string {
			s := d.(ServiceState)
			return []string{
				fmt.Sprintf("%v", index),
				s.Type,
				s.Name,
				s.StatusString(),
				s.ListenURL,
				s.TargetURL,
				fmt.Sprintf("%v", s.GetActiveCount()),
				fmt.Sprintf("%v", s.GetDoneCount()),
			}
		},
	)
	return buf.String()
}
