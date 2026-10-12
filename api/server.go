package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/utils"
)

type Server struct {
	backend Backend
	log     *slog.Logger
	server  *http.Server
	router  *mux.Router
	wsHub   *WSHub
	poller  *StatePoller

	unsubNet func()
	unsubSvc func()
}

func New(backend Backend, cfg configv2.APIInfo, log *slog.Logger) *Server {
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
		backend: backend,
		log:     log,
		router:  router,
		wsHub:   wsHub,
		server: &http.Server{
			Addr:    listen,
			Handler: corsMiddleware(router),
		},
		poller: NewStatePoller(backend, wsHub, time.Duration(pollInterval)*time.Second),
	}

	s.registerRoutes()
	return s
}

// corsMiddleware 允许本地前端跨域访问 API
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Confirm")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) registerRoutes() {
	r := s.router.PathPrefix("/api/v1").Subrouter()

	r.HandleFunc("/status", s.handleStatus).Methods("GET")
	r.HandleFunc("/networks", s.handleNetworks).Methods("GET")
	r.HandleFunc("/services", s.handleServices).Methods("GET")
	r.HandleFunc("/system/listening-ports", s.handleListeningPorts).Methods("GET")
	r.HandleFunc("/streams", s.handleStreams).Methods("GET")
	r.HandleFunc("/ping", s.handlePingAll).Methods("POST")
	r.HandleFunc("/ping/{network}/{domain}", s.handlePingSingle).Methods("POST")
	r.HandleFunc("/loglevel", s.handleLogLevel).Methods("PUT")
	r.HandleFunc("/stop", s.handleStop).Methods("POST")
	r.HandleFunc("/links", s.handleAddLink).Methods("POST")
	r.HandleFunc("/tunnels", s.handleAddTunnel).Methods("POST")
	r.HandleFunc("/ws", s.handleWebSocket)
}

// Start 非阻塞启动 API 服务器
func (s *Server) Start() error {
	// 订阅后端事件并泵送到 WebSocket
	var netCh <-chan NetworkStateEvent
	var svcCh <-chan ServiceStatusEvent
	netCh, s.unsubNet = s.backend.SubscribeNetworkState(64)
	svcCh, s.unsubSvc = s.backend.SubscribeServiceStatus(64)
	go s.eventLoop(netCh, svcCh)

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
	if s.unsubNet != nil {
		s.unsubNet()
	}
	if s.unsubSvc != nil {
		s.unsubSvc()
	}
	s.poller.Stop()
	s.wsHub.CloseAll()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		s.log.Error("api server shutdown error", "err", err)
	}
	s.log.Info("api server stopped")
}

// eventLoop 把后端事件翻译为 WebSocket 广播
func (s *Server) eventLoop(netCh <-chan NetworkStateEvent, svcCh <-chan ServiceStatusEvent) {
	for netCh != nil || svcCh != nil {
		select {
		case evt, ok := <-netCh:
			if !ok {
				netCh = nil
				continue
			}
			s.broadcastNetworkState(evt)
		case evt, ok := <-svcCh:
			if !ok {
				svcCh = nil
				continue
			}
			s.broadcastServiceStatus(evt)
		}
	}
}

func (s *Server) broadcastNetworkState(evt NetworkStateEvent) {
	// 获取最新的网络报告
	var report *NetworkStateDTO
	reports, err := s.backend.GetAllNetworkState()
	if err == nil {
		for _, r := range reports {
			if r.Name == evt.Network {
				dto := toNetworkDTO(r)
				report = &dto
				break
			}
		}
	}

	s.wsHub.Broadcast("network.state", map[string]interface{}{
		"name":     evt.Network,
		"oldState": evt.OldState,
		"newState": evt.NewState,
		"report":   report,
	})
}

func (s *Server) broadcastServiceStatus(evt ServiceStatusEvent) {
	var service *ServiceStateDTO
	states, err := s.backend.GetAllServiceState()
	if err == nil {
		for _, st := range states {
			if st.Name == evt.Name {
				dto := toServiceDTO(st)
				service = &dto
				break
			}
		}
	}

	s.wsHub.Broadcast("service.status", map[string]interface{}{
		"name":      evt.Name,
		"oldStatus": evt.OldStatus,
		"newStatus": evt.NewStatus,
		"service":   service,
	})
}
