package vservice

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/net-agent/remotework/utils"
)

// ServiceSupervisor 管理服务的生命周期（Init → Start → Stop）
type ServiceSupervisor struct {
	log             *slog.Logger
	registry        *ServiceRegistry
	waiter          sync.WaitGroup
	running         int32                                                 // atomic: 0=stopped, 1=running
	OnStatusChange  func(name string, oldStatus, newStatus ServiceStatus) // 可选回调
	IsDependencyErr func(error) bool                                      // 可选：判断错误是否为依赖未就绪
}

func NewServiceSupervisor(log *slog.Logger, registry *ServiceRegistry) *ServiceSupervisor {
	if log == nil {
		log = utils.NewModuleLogger("hub.svc")
	}
	return &ServiceSupervisor{
		log:      log,
		registry: registry,
	}
}

func (sv *ServiceSupervisor) StartAll() error {
	if !atomic.CompareAndSwapInt32(&sv.running, 0, 1) {
		return errors.New("service is running")
	}
	defer atomic.StoreInt32(&sv.running, 0)

	sv.log.Info("start services")
	sv.registry.Range(func(svc *Service) {
		sv.Start(svc)
	})

	sv.waiter.Wait()
	sv.log.Info("no service is running")

	// 统计 Init 失败的服务
	var failed, total int
	sv.registry.Range(func(svc *Service) {
		total++
		if svc.GetStatus() == StatusFailed {
			failed++
		}
	})
	if failed > 0 {
		return fmt.Errorf("%d/%d services failed to init", failed, total)
	}
	return nil
}

func (sv *ServiceSupervisor) Start(svc *Service) {
	st := svc.GetStatus()
	if st == StatusInit || st == StatusRunning {
		return
	}

	sv.waiter.Add(1)
	go sv.manageState(svc, &sv.waiter)
}

func (sv *ServiceSupervisor) manageState(svc *Service, waiter *sync.WaitGroup) {
	defer waiter.Done()
	sv.log.Info("init service", "type", svc.Type, "name", svc.Name)

	oldStatus := svc.GetStatus()
	svc.SetStatus(StatusInit)
	sv.fireStatusChange(svc.Name, oldStatus, StatusInit)

	if err := svc.Controller.Init(); err != nil {
		if sv.IsDependencyErr != nil && sv.IsDependencyErr(err) {
			svc.SetStatus(StatusPending)
			sv.fireStatusChange(svc.Name, StatusInit, StatusPending)
			sv.log.Info("service pending", "name", svc.Name, "err", err)
			return
		}
		svc.LastErr = err.Error()
		svc.SetStatus(StatusFailed)
		sv.fireStatusChange(svc.Name, StatusInit, StatusFailed)
		sv.log.Error("init service failed", "name", svc.Name, "err", err)
		return
	}

	svc.SetStatus(StatusRunning)
	sv.fireStatusChange(svc.Name, StatusInit, StatusRunning)

	err := svc.Controller.Start()

	svc.SetStatus(StatusStopped)
	sv.fireStatusChange(svc.Name, StatusRunning, StatusStopped)

	sv.log.Info("service stopped", "name", svc.Name, "err", err)
}

func (sv *ServiceSupervisor) fireStatusChange(name string, oldStatus, newStatus ServiceStatus) {
	if sv.OnStatusChange != nil {
		sv.OnStatusChange(name, oldStatus, newStatus)
	}
}

func (sv *ServiceSupervisor) StopAll() {
	if atomic.LoadInt32(&sv.running) == 0 {
		return
	}

	sv.registry.Range(func(svc *Service) {
		if svc.GetStatus() == StatusRunning {
			svc.Controller.Close()
		}
	})
}

func (sv *ServiceSupervisor) IsRunning() bool {
	return atomic.LoadInt32(&sv.running) == 1
}
