package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

type Server struct {
	hub    *agent.Hub
	log    *slog.Logger
	server *http.Server
	router *mux.Router
	wsHub  *WSHub
	poller *StatePoller
}

func New(hub *agent.Hub, cfg agent.APIInfo, log *slog.Logger) *Server {
	if log == nil {
		log = utils.NewModuleLogger("api")
	}

	listen := cfg.Listen
	if listen == "" {
		listen = "127.0.0.1:8080"
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5
	}

	router := mux.NewRouter()
	wsHub := NewWSHub(log.With("module", "api.ws"))

	s := &Server{
		hub:   hub,
		log:   log,
		router: router,
		wsHub: wsHub,
		server: &http.Server{
			Addr:    listen,
			Handler: router,
		},
		poller: NewStatePoller(hub, wsHub, time.Duration(pollInterval)*time.Second),
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	r := s.router.PathPrefix("/api/v1").Subrouter()

	r.HandleFunc("/status", s.handleStatus).Methods("GET")
	r.HandleFunc("/networks", s.handleNetworks).Methods("GET")
	r.HandleFunc("/services", s.handleServices).Methods("GET")
	r.HandleFunc("/streams", s.handleStreams).Methods("GET")
	r.HandleFunc("/ping", s.handlePingAll).Methods("POST")
	r.HandleFunc("/ping/{network}/{domain}", s.handlePingSingle).Methods("POST")
	r.HandleFunc("/loglevel", s.handleLogLevel).Methods("PUT")
	r.HandleFunc("/stop", s.handleStop).Methods("POST")
	r.HandleFunc("/ws", s.handleWebSocket)
}

// Start 非阻塞启动 API 服务器
func (s *Server) Start() error {
	s.hub.AddEventListener(s)
	s.poller.Start()

	go func() {
		s.log.Info("api server starting", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("api server error", "err", err)
		}
	}()
	return nil
}

// Stop 优雅关闭
func (s *Server) Stop() {
	s.hub.RemoveEventListener(s)
	s.poller.Stop()
	s.wsHub.CloseAll()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		s.log.Error("api server shutdown error", "err", err)
	}
	s.log.Info("api server stopped")
}

// OnNetworkStateChange 实现 HubEventListener 接口
func (s *Server) OnNetworkStateChange(name, oldState, newState string) {
	// 获取最新的网络报告
	var report *NetworkStateDTO
	reports, err := s.hub.GetAllNetworkState()
	if err == nil {
		for _, r := range reports {
			if r.Name == name {
				dto := toNetworkDTO(r)
				report = &dto
				break
			}
		}
	}

	s.wsHub.Broadcast("network.state", map[string]interface{}{
		"name":     name,
		"oldState": oldState,
		"newState": newState,
		"report":   report,
	})
}

// OnServiceStatusChange 实现 HubEventListener 接口
func (s *Server) OnServiceStatusChange(name string, oldStatus, newStatus agent.ServiceStatus) {
	var service *ServiceStateDTO
	states, err := s.hub.GetAllServiceState()
	if err == nil {
		for _, st := range states {
			if st.Name == name {
				dto := toServiceDTO(st)
				service = &dto
				break
			}
		}
	}

	s.wsHub.Broadcast("service.status", map[string]interface{}{
		"name":      name,
		"oldStatus": oldStatus.String(),
		"newStatus": newStatus.String(),
		"service":   service,
	})
}
