package server

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/net-agent/flex/v3/packet"
	"github.com/net-agent/flex/v3/switcher"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewModuleLogger("server")

// Server 封装 relay 服务的完整生命周期
type Server struct {
	config *Config
	app    *switcher.Server
	mux    *Mux
}

func NewServer(config *Config) *Server {
	app := switcher.NewServer(config.Server.Password, syslog, nil)
	app.OnContextStart = func(ctx *switcher.Context) {
		syslog.Info("agent connected", "domain", ctx.Domain, "ip", ctx.IP)
	}
	app.OnContextStop = func(ctx *switcher.Context, duration time.Duration) {
		syslog.Info("agent disconnected", "domain", ctx.Domain, "ip", ctx.IP, "duration", duration)
	}
	return &Server{config: config, app: app}
}

// ListenAndServe 监听端口并启动服务，阻塞直到出错或 Close 被调用
func (s *Server) ListenAndServe() error {
	l, err := net.Listen("tcp", s.config.Server.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	syslog.Info("server started", "addr", s.config.Server.Listen)

	s.mux = NewMux(l, syslog)
	go s.app.Serve(s.mux.FlexListener())

	if s.config.Server.WsEnable {
		r := mux.NewRouter()
		r.Methods("GET").Path(s.config.Server.WsPath).HandlerFunc(s.wsHandler())
		go http.Serve(s.mux.HTTPListener(), r)
	}

	return s.mux.Serve()
}

func (s *Server) Close() error {
	if s.mux != nil {
		return s.mux.Close()
	}
	return nil
}

func (s *Server) wsHandler() http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Fprintf(w, "upgrade failed: %v", err)
			return
		}
		pc := packet.NewWithWs(c)
		syslog.Info("ws agent connected", "remote", c.RemoteAddr())
		go s.app.ServeConn(pc)
	}
}

// RunServer 从配置文件启动服务（保持向后兼容）
func RunServer(configName string) {
	resolved, err := utils.ResolveConfigFile(configName)
	if err != nil {
		utils.Fatal(syslog, "load config failed: ", err)
	}
	syslog.Info("read config", "path", resolved)

	config, err := NewConfig(resolved)
	if err != nil {
		utils.Fatal(syslog, "load config failed: ", err)
	}

	srv := NewServer(config)
	if err := srv.ListenAndServe(); err != nil {
		syslog.Error("server stopped", "error", err)
	}
}
