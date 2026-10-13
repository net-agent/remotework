package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/utils"
)

// Config 服务器运行时选项（与文件配置解耦，由调用方组装）
type Config struct {
	Listen       string // 监听地址，缺省 127.0.0.1:8080；":0" 表示端口自选
	PollInterval int    // 数据流轮询间隔（秒），缺省 5
	Token        string // Bearer token，空表示 standalone 开放模式
	Meta         Meta   // /status 元信息
}

// Meta 通过 /status 对外回报的进程元信息
type Meta struct {
	Protocol    int    // sidecar 协议版本
	Version     string // 构建版本（ldflags 注入）
	Commit      string // 构建 commit
	Mode        string // "normal" | "degraded"
	ConfigPath  string // 配置文件绝对路径
	ConfigError string // 降级模式时的配置错误汇总
}

type Server struct {
	backend   Backend
	log       *slog.Logger
	server    *http.Server
	router    *mux.Router
	wsHub     *WSHub
	poller    *StatePoller
	token     string
	meta      Meta
	startedAt time.Time

	addr atomic.Value // string，实际绑定地址，Start 成功后可读

	unsubNet func()
	unsubSvc func()
}

func New(backend Backend, cfg Config, log *slog.Logger) *Server {
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

	if cfg.Token == "" {
		log.Warn("api server running without auth token")
	}

	router := mux.NewRouter()
	wsHub := NewWSHub(log.With("module", "api.ws"))

	s := &Server{
		backend:   backend,
		log:       log,
		router:    router,
		wsHub:     wsHub,
		token:     cfg.Token,
		meta:      cfg.Meta,
		startedAt: time.Now(),
		poller:    NewStatePoller(backend, wsHub, time.Duration(pollInterval)*time.Second),
	}
	s.server = &http.Server{
		Addr:    listen,
		Handler: hostCheckMiddleware(corsMiddleware(s.authMiddleware(router))),
	}

	s.registerRoutes()
	return s
}

// writeError 以指定 HTTP 状态码输出统一错误信封
func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	utils.WriteJSON(w, err, nil)
}

// allowedHosts Host 头白名单（防 DNS rebinding），匹配前去除端口
var allowedHosts = map[string]bool{
	"127.0.0.1": true,
	"localhost": true,
	"::1":       true,
	"[::1]":     true,
}

// hostCheckMiddleware 校验 Host 头必须在本地白名单内
func hostCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !allowedHosts[host] {
			writeError(w, http.StatusForbidden, errors.New("forbidden host"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedOrigins CORS / WebSocket Origin 白名单
var allowedOrigins = map[string]bool{
	"tauri://localhost":      true, // macOS 生产
	"http://tauri.localhost": true, // Windows 生产
	"http://localhost:1420":  true, // dev
}

// corsMiddleware 收敛为 Origin 白名单：命中则反射 Origin，未命中不放行跨域
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware Token 非空时校验 Authorization: Bearer <token>；
// WebSocket 端点由 handleWebSocket 自行校验 ?token=（webview 无法自定义 WS 请求头）
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.URL.Path == "/api/v1/ws" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+s.token {
			writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
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

// Start 同步绑定监听端口，绑定失败立即返回错误；
// 成功后保存实际地址并异步服务
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	s.addr.Store(ln.Addr().String())

	// 订阅后端事件并泵送到 WebSocket
	var netCh <-chan NetworkStateEvent
	var svcCh <-chan ServiceStatusEvent
	netCh, s.unsubNet = s.backend.SubscribeNetworkState(64)
	svcCh, s.unsubSvc = s.backend.SubscribeServiceStatus(64)
	go s.eventLoop(netCh, svcCh)

	s.poller.Start()

	go func() {
		s.log.Info("api server started", "addr", s.Addr())
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error("api server error", "err", err)
		}
	}()
	return nil
}

// Addr 返回实际绑定的监听地址；Start 成功前返回空串
func (s *Server) Addr() string {
	if v, ok := s.addr.Load().(string); ok {
		return v
	}
	return ""
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
