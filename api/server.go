package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/utils"
)

// HTTP 面超时：控制面请求都是小对象，短超时即可覆盖；
// IdleTimeout 允许桌面端复用 keep-alive 连接。
const (
	apiReadHeaderTimeout = 5 * time.Second
	apiReadTimeout       = 15 * time.Second
	apiWriteTimeout      = 30 * time.Second
	apiIdleTimeout       = 60 * time.Second
)

const (
	// eventLoopStopTimeout Stop 等待事件循环退出的上限（退订通道不关闭时兜底）
	eventLoopStopTimeout = 2 * time.Second
	// listeningPortsCacheTTL /system/listening-ports 的采集缓存时长
	listeningPortsCacheTTL = 2 * time.Second
	// pingMaxConcurrent /ping 同时执行的后台探测数上限
	pingMaxConcurrent = 4
)

// ping 整请求等待上限（后端探测内部还有 3s/domain 超时）；变量便于测试缩短
var (
	pingRequestTimeout = 8 * time.Second
	pingSingleTimeout  = 5 * time.Second
)

// Config 服务器运行时选项（与文件配置解耦，由调用方组装）
type Config struct {
	Listen              string // 监听地址，缺省 127.0.0.1:8080；":0" 表示端口自选
	PollInterval        int    // 网络/服务轮询间隔（秒），缺省 5
	StreamStatsInterval int    // 数据流快照推送间隔（秒），缺省 1
	Token               string // Bearer token，空表示 standalone 开放模式
	AllowNonLoopback    bool   // 显式允许非回环监听（仍强制要求 Token）
	Meta                Meta   // /status 元信息
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
	backend          Backend
	log              *slog.Logger
	server           *http.Server
	router           *mux.Router
	wsHub            *WSHub
	poller           *StatePoller
	meter            *streamMeter        // 流速率计，REST 与轮询器共享
	resolver         *clientProcResolver // 客户端进程归属解析器，REST/快照/轮询/事件共享
	token            string
	allowNonLoopback bool
	nonLoopback      atomic.Bool
	meta             Meta
	startedAt        time.Time

	addr atomic.Value // string，实际绑定地址，Start 成功后可读

	// 生命周期：Stop 幂等；stopCh 用于唤醒 eventLoop（退订通道可能不关闭）
	stopCh    chan struct{}
	loopDone  chan struct{}
	stopOnce  sync.Once
	started   atomic.Bool
	pingSem   chan struct{}
	portsMu   sync.Mutex
	portsAt   time.Time
	portsData []ListeningPortDTO
	portsOK   bool
	listPorts func() ([]utils.ListeningPortInfo, error)

	unsubNet    func()
	unsubSvc    func()
	unsubStream func()
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
	statsInterval := cfg.StreamStatsInterval
	if statsInterval <= 0 {
		statsInterval = 1
	}

	if cfg.Token == "" {
		log.Warn("api server running without auth token; only loopback listeners are allowed")
	}

	router := mux.NewRouter()
	wsHub := NewWSHub(log.With("module", "api.ws"))
	meter := newStreamMeter()
	resolver := newClientProcResolver()

	s := &Server{
		backend:          backend,
		log:              log,
		router:           router,
		wsHub:            wsHub,
		token:            cfg.Token,
		allowNonLoopback: cfg.AllowNonLoopback,
		meta:             cfg.Meta,
		startedAt:        time.Now(),
		meter:            meter,
		resolver:         resolver,
		stopCh:           make(chan struct{}),
		loopDone:         make(chan struct{}),
		pingSem:          make(chan struct{}, pingMaxConcurrent),
		listPorts:        utils.ListListeningPorts,
		poller:           NewStatePoller(backend, wsHub, meter, resolver, time.Duration(pollInterval)*time.Second, time.Duration(statsInterval)*time.Second),
	}
	s.server = &http.Server{
		Addr:              listen,
		Handler:           s.hostCheckMiddleware(corsMiddleware(s.authMiddleware(router))),
		ReadHeaderTimeout: apiReadHeaderTimeout,
		ReadTimeout:       apiReadTimeout,
		WriteTimeout:      apiWriteTimeout,
		IdleTimeout:       apiIdleTimeout,
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

// hostCheckMiddleware 校验 Host 头。默认只允许 loopback（防 DNS rebinding）；
// 显式开启非回环监听且配置了 token 时，允许任意 Host——此时认证由 token
// 承担，浏览器无法在不知道 token 的情况下利用 DNS rebinding 发起有效请求。
func (s *Server) hostCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !allowedHosts[host] {
			if !(s.nonLoopback.Load() && s.token != "") {
				writeError(w, http.StatusForbidden, errors.New("forbidden host"))
				return
			}
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

// isSafeMethod 幂等方法不改变服务端状态，可只依赖 CORS 响应头限制读取
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// corsMiddleware 收敛为 Origin 白名单：
//   - 命中白名单的跨域请求反射 Origin 并放行预检；
//   - 非浏览器请求（无 Origin）照常放行，兼容 CLI / sidecar；
//   - 带 Origin 的写请求（POST/PUT/DELETE...）只要未命中白名单就直接 403。
//     仅省略 ACAO 头不足以阻止 CSRF——简单请求仍会被浏览器发出并生效。
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := origin != "" && allowedOrigins[origin]
		if allowed {
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if origin != "" && !allowed && !isSafeMethod(r.Method) {
			writeError(w, http.StatusForbidden, errors.New("forbidden origin"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// secureEqual 常量时间字符串比较，避免 token 比较的时序侧信道
func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// authMiddleware Token 非空时校验 Authorization: Bearer <token>；
// WebSocket 端点由 handleWebSocket 自行校验 ?token=（webview 无法自定义 WS 请求头）
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.URL.Path == "/api/v1/ws" {
			next.ServeHTTP(w, r)
			return
		}
		if !secureEqual(r.Header.Get("Authorization"), "Bearer "+s.token) {
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
	r.HandleFunc("/streams/{network}/{id}", s.handleCloseStream).Methods("DELETE")
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
	// 无 token 的开放模式只允许回环监听；非回环监听必须显式开启
	// allowNonLoopback 且配置 token，防止把控制面意外暴露到局域网/公网。
	loopback := isLoopbackAddr(ln.Addr())
	if !loopback {
		if s.token == "" {
			ln.Close()
			return fmt.Errorf("refusing to serve API without auth token on non-loopback address %q", ln.Addr())
		}
		if !s.allowNonLoopback {
			ln.Close()
			return fmt.Errorf("refusing non-loopback API address %q without allowNonLoopback=true", ln.Addr())
		}
	}
	s.nonLoopback.Store(!loopback)
	s.addr.Store(advertiseAddr(ln.Addr()))

	// 通配监听（0.0.0.0/[::]）时向宿主回报 loopback 地址：ready 行是给本机
	// 桌面端连接用的，[...]:port 不可移植。监听本身仍保持通配。

	// 订阅后端事件并泵送到 WebSocket
	var netCh <-chan NetworkStateEvent
	var svcCh <-chan ServiceStatusEvent
	var streamCh <-chan StreamLifecycleEvent
	netCh, s.unsubNet = s.backend.SubscribeNetworkState(64)
	svcCh, s.unsubSvc = s.backend.SubscribeServiceStatus(64)
	streamCh, s.unsubStream = s.backend.SubscribeStreamEvents(256)
	s.started.Store(true)
	go s.eventLoop(netCh, svcCh, streamCh)

	s.poller.Start()

	go func() {
		s.log.Info("api server started", "addr", s.Addr())
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error("api server error", "err", err)
		}
	}()
	return nil
}

// advertiseAddr 把通配监听地址转换为本机可连接的 loopback 地址；
// 其他地址原样返回。仅用于 Addr()/ready 行，不改变实际绑定。
func advertiseAddr(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if ok && (tcp.IP == nil || tcp.IP.IsUnspecified()) {
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(tcp.Port))
	}
	return addr.String()
}

// isLoopbackAddr 判断监听地址是否绑定回环网卡
func isLoopbackAddr(addr net.Addr) bool {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || tcp.IP == nil {
		return false
	}
	return tcp.IP.IsLoopback()
}

// acquirePingSlot 非阻塞申请一个 ping 后台槽位；满则返回 false（快速失败不排队）
func (s *Server) acquirePingSlot() bool {
	select {
	case s.pingSem <- struct{}{}:
		return true
	default:
		return false
	}
}

// releasePingSlot 归还 ping 后台槽位
func (s *Server) releasePingSlot() {
	<-s.pingSem
}

// Addr 返回实际绑定的监听地址；Start 成功前返回空串
func (s *Server) Addr() string {
	if v, ok := s.addr.Load().(string); ok {
		return v
	}
	return ""
}

// Stop 优雅关闭；幂等（重复调用直接返回）。
// 事件循环通过 stopCh 主动停止，不依赖后端退订时关闭 channel
// （Catalog 退订通道是否关闭由 kernel 决定，此处不能假设）。
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		// 先广播 hub.stopped 并留出发送窗口，比轮询检测更即时
		s.wsHub.Broadcast("hub.stopped", map[string]interface{}{})
		time.Sleep(100 * time.Millisecond)

		close(s.stopCh)

		if s.unsubNet != nil {
			s.unsubNet()
		}
		if s.unsubSvc != nil {
			s.unsubSvc()
		}
		if s.unsubStream != nil {
			s.unsubStream()
		}
		s.poller.Stop()

		if s.started.Load() {
			select {
			case <-s.loopDone:
			case <-time.After(eventLoopStopTimeout):
				s.log.Warn("api event loop did not stop in time")
			}
		}
		s.wsHub.CloseAll()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.log.Error("api server shutdown error", "err", err)
		}
		s.log.Info("api server stopped")
	})
}

// eventLoop 把后端事件翻译为 WebSocket 广播；
// 流生命周期事件经批处理器按窗口合并，避免洪流场景逐条推送
func (s *Server) eventLoop(netCh <-chan NetworkStateEvent, svcCh <-chan ServiceStatusEvent, streamCh <-chan StreamLifecycleEvent) {
	defer close(s.loopDone)
	batcher := newStreamEventBatcher()
	flushTick := time.NewTicker(streamEventBatchWindow)
	defer flushTick.Stop()

	for netCh != nil || svcCh != nil || streamCh != nil {
		select {
		case <-s.stopCh:
			batcher.flush(s.wsHub, s.backend, s.resolver)
			return
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
		case evt, ok := <-streamCh:
			if !ok {
				streamCh = nil
				continue
			}
			if evt.State != nil {
				batcher.add(evt)
			}
		case <-flushTick.C:
			batcher.flush(s.wsHub, s.backend, s.resolver)
		}
	}
	// 事件源全部关闭，冲刷残余批量事件
	batcher.flush(s.wsHub, s.backend, s.resolver)
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
