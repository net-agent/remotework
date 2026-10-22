package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v3/packet/ws"
	"github.com/net-agent/flex/v3/switcher"
	"github.com/net-agent/remotework/utils"
)

// Server 封装 relay 服务的完整生命周期
type Server struct {
	config *Config
	app    *switcher.Server
	mux    *Mux
	http   *http.Server
	log    *slog.Logger
}

func NewServer(config *Config) *Server {
	log := utils.NewModuleLogger("server")
	app := switcher.NewServer(config.Server.Password, log, nil)
	fair := switcher.DefaultSchedulerConfig()
	log.Info("enabling fair connection scheduling",
		"enabled", true,
		"policy", schedulerPolicyName(fair.Policy),
		"quantumBytes", fair.QuantumBytes,
		"minChargeBytes", fair.MinChargeBytes,
		"quantumPackets", fair.QuantumPackets,
	)
	app.OnContextStart = func(ctx *switcher.Context) {
		log.Info("agent connected", "domain", ctx.Domain, "ip", ctx.IP)
	}
	app.OnContextStop = func(ctx *switcher.Context, duration time.Duration) {
		log.Info("agent disconnected", "domain", ctx.Domain, "ip", ctx.IP, "duration", duration)
	}
	return &Server{config: config, app: app, log: log}
}

// ListenAndServe 监听端口并启动服务，阻塞直到出错或 Close 被调用。
// 返回 nil 表示被 Close 正常停止，返回 error 表示启动/运行失败。
func (s *Server) ListenAndServe() error {
	l, err := net.Listen("tcp", s.config.Server.Listen)
	if err != nil {
		return fmt.Errorf("listen %q: %w", s.config.Server.Listen, err)
	}
	s.log.Info("server started", "addr", l.Addr().String())

	s.mux = NewMux(l, s.log)
	go func() {
		if err := s.app.Serve(s.mux.FlexListener()); err != nil {
			s.log.Error("flex serve stopped", "err", err)
		}
	}()

	if s.config.Server.WsEnable {
		r := mux.NewRouter()
		r.Methods("GET").Path(s.config.Server.WsPath).HandlerFunc(s.wsHandler())
		httpServer := &http.Server{
			Handler:           r,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		s.http = httpServer
		go func() {
			if err := httpServer.Serve(s.mux.HTTPListener()); err != nil && err != http.ErrServerClosed {
				s.log.Error("http serve stopped", "err", err)
			}
		}()
	} else {
		// WsEnable=false 时没有任何消费者读取 HTTP 连接；关闭通道让分流器
		// 直接丢弃 HTTP 连接，而不是在 Offer 上永久阻塞。
		s.mux.CloseHTTP()
	}

	err = s.mux.Serve()
	// Mux.Serve 在 Close 场景返回 net.ErrClosed，视为正常停止。
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func (s *Server) Close() error {
	var err error
	if s.http != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.http.Shutdown(ctx)
		cancel()
	}
	if s.mux != nil {
		err = s.mux.Close()
	}
	if s.app != nil {
		_ = s.app.Close()
	}
	return err
}

func (s *Server) wsHandler() http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// 不把内部错误回显给未认证客户端
			http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
			s.log.Warn("ws upgrade failed", "remote", r.RemoteAddr, "err", err)
			return
		}
		pc := ws.NewConn(c)
		s.log.Info("ws agent connected", "remote", c.RemoteAddr())
		go func() {
			if err := s.app.ServeConn(pc); err != nil {
				s.log.Debug("ws agent disconnected", "remote", c.RemoteAddr(), "err", err)
			}
		}()
	}
}

// RunServer 从配置文件启动服务，处理 SIGINT/SIGTERM 做优雅关闭。
// 配置/绑定错误返回 error，由 main 负责非零退出。
func RunServer(configName string) error {
	log := utils.NewModuleLogger("server")

	resolved, err := utils.ResolveConfigFile(configName)
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}
	log.Info("read config", "path", resolved)

	config, err := NewConfig(resolved)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	srv := NewServer(config)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		log.Info("shutdown signal received", "signal", sig)
		go func() {
			s := <-sigCh
			log.Warn("second signal received, forcing exit", "signal", s)
			os.Exit(130)
		}()
		_ = srv.Close()
		select {
		case <-errCh:
		case <-time.After(5 * time.Second):
			return fmt.Errorf("server shutdown timed out")
		}
		return nil
	}
}

// schedulerPolicyName 把 flex 的调度策略枚举转成可读名称。
func schedulerPolicyName(p switcher.SchedulerPolicy) string {
	if p == switcher.SchedulerPolicyPackets {
		return "packets"
	}
	return "cost-budget"
}
