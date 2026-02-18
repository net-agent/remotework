package agent

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/net-agent/remotework/utils"
)

// ServiceManager 管理所有服务的注册、生命周期和状态
type ServiceManager struct {
	log     *slog.Logger
	svcs    []*Service
	names   map[string]*Service
	mut     sync.RWMutex
	id      int32
	waiter  sync.WaitGroup
	running int32 // atomic: 0=stopped, 1=running
}

func NewServiceManager(log *slog.Logger) *ServiceManager {
	if log == nil {
		log = utils.NewModuleLogger("hub.svc")
	}
	return &ServiceManager{
		log:   log,
		names: make(map[string]*Service),
	}
}

func (sm *ServiceManager) Add(svc *Service) error {
	svc.ID = atomic.AddInt32(&sm.id, 1)

	sm.mut.Lock()
	defer sm.mut.Unlock()

	if _, found := sm.names[svc.Name]; found {
		return errors.New("duplicate service name")
	}

	svc.SetStatus(StatusUninit)
	sm.svcs = append(sm.svcs, svc)
	sm.names[svc.Name] = svc

	return nil
}

func (sm *ServiceManager) Find(name string) (*Service, error) {
	sm.mut.RLock()
	defer sm.mut.RUnlock()

	svc, found := sm.names[name]
	if !found {
		return nil, errors.New("service not found")
	}
	return svc, nil
}

func (sm *ServiceManager) Range(fn func(svc *Service)) {
	sm.mut.RLock()
	svcs := make([]*Service, len(sm.svcs))
	copy(svcs, sm.svcs)
	sm.mut.RUnlock()

	for _, svc := range svcs {
		fn(svc)
	}
}

func (sm *ServiceManager) StartAll() error {
	if !atomic.CompareAndSwapInt32(&sm.running, 0, 1) {
		return errors.New("service is running")
	}
	defer atomic.StoreInt32(&sm.running, 0)

	sm.log.Info("start services")
	for _, svc := range sm.svcs {
		sm.Start(svc)
	}

	sm.waiter.Wait()
	sm.log.Info("no service is running")

	// 统计 Init 失败的服务
	var failed int
	for _, svc := range sm.svcs {
		if svc.GetStatus() == StatusFailed {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d services failed to init", failed, len(sm.svcs))
	}
	return nil
}

func (sm *ServiceManager) Start(svc *Service) {
	st := svc.GetStatus()
	if st == StatusInit || st == StatusRunning {
		return
	}

	sm.waiter.Add(1)
	go sm.manageState(svc, &sm.waiter)
}

func (sm *ServiceManager) manageState(svc *Service, waiter *sync.WaitGroup) {
	defer waiter.Done()
	sm.log.Info("init service", "type", svc.Type, "name", svc.Name)

	svc.SetStatus(StatusInit)
	if err := svc.controller.Init(); err != nil {
		svc.SetStatus(StatusFailed)
		sm.log.Error("init service failed", "name", svc.Name, "err", err)
		return
	}

	svc.SetStatus(StatusRunning)
	err := svc.controller.Start()
	svc.SetStatus(StatusStopped)

	sm.log.Info("service stopped", "name", svc.Name, "err", err)
}

func (sm *ServiceManager) StopAll() {
	if atomic.LoadInt32(&sm.running) == 0 {
		return
	}

	sm.mut.RLock()
	svcs := make([]*Service, len(sm.svcs))
	copy(svcs, sm.svcs)
	sm.mut.RUnlock()

	for _, svc := range svcs {
		if svc.GetStatus() == StatusRunning {
			svc.controller.Close()
		}
	}
}

func (sm *ServiceManager) IsRunning() bool {
	return atomic.LoadInt32(&sm.running) == 1
}

// UpdateByNetwork 遍历服务，找到依赖指定网络的服务并触发更新
func (sm *ServiceManager) UpdateByNetwork(network string) {
	count := 0
	sm.mut.RLock()
	svcs := make([]*Service, len(sm.svcs))
	copy(svcs, sm.svcs)
	sm.mut.RUnlock()

	for _, svc := range svcs {
		if svc.IsListenDepend(network) && svc.GetStatus() == StatusRunning {
			go svc.controller.Update()
			count++
		}
	}
	sm.log.Info("update network", "network", network, "updated", count)
}

// Names 返回所有已注册的服务名称
func (sm *ServiceManager) Names() []string {
	sm.mut.RLock()
	defer sm.mut.RUnlock()

	names := make([]string, 0, len(sm.names))
	for name := range sm.names {
		names = append(names, name)
	}
	return names
}
