package utils

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	_ "net/http/pprof" // 导入 pprof 包以注册路由
	"strings"
	"sync"
	"time"
)

// pprofServerTimeouts pprof 端点可能执行 30s 的 CPU profile，超时需留足余量。
const (
	pprofReadHeaderTimeout = 5 * time.Second
	pprofReadTimeout       = 60 * time.Second
	pprofWriteTimeout      = 60 * time.Second
	pprofIdleTimeout       = 60 * time.Second
)

// PprofServer 管理 pprof HTTP 服务器。
//
// 生命周期契约：Start 同步完成 loopback 校验与端口绑定（bind 失败立即返回错误），
// 成功后后台 Serve；Stop 幂等。pprof 携带堆/配置等敏感信息，只允许绑定 loopback：
// 未指定 host 时强制 127.0.0.1，显式指定非 loopback 地址则拒绝启动。
type PprofServer struct {
	mu     sync.Mutex
	server *http.Server
	log    *slog.Logger
}

// NewPprofServer 创建新的 pprof 服务器实例
func NewPprofServer(log *slog.Logger) *PprofServer {
	if log == nil {
		log = NewModuleLogger("pprof")
	}
	return &PprofServer{
		log: log,
	}
}

// Start 启动 pprof HTTP 服务器（同步 bind，返回错误）
func (p *PprofServer) Start(addr string) error {
	if addr == "" {
		err := errors.New("pprof listen address cannot be empty")
		p.log.Error("start pprof server failed", "err", err)
		return err
	}

	bindAddr, err := pprofLoopbackAddr(addr)
	if err != nil {
		p.log.Error("refusing to start pprof server", "addr", addr, "err", err)
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 重复启动：先关掉旧实例，避免 listener 泄漏
	if p.server != nil {
		_ = p.server.Close()
		p.server = nil
	}

	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		p.log.Error("start pprof server failed", "addr", bindAddr, "err", err)
		return fmt.Errorf("pprof listen %s: %w", bindAddr, err)
	}

	srv := &http.Server{
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: pprofReadHeaderTimeout,
		ReadTimeout:       pprofReadTimeout,
		WriteTimeout:      pprofWriteTimeout,
		IdleTimeout:       pprofIdleTimeout,
	}
	p.server = srv

	go func() {
		p.log.Info("starting pprof server", "addr", ln.Addr().String())
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			p.log.Error("pprof server error", "err", err)
		}
	}()

	return nil
}

// Stop 停止 pprof HTTP 服务器；重复调用为空操作
func (p *PprofServer) Stop() {
	p.mu.Lock()
	srv := p.server
	p.server = nil
	p.mu.Unlock()

	if srv != nil {
		_ = srv.Close()
		p.log.Info("pprof server stopped")
	}
}

// IsRunning 检查 pprof 服务器是否正在运行
func (p *PprofServer) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.server != nil
}

// pprofLoopbackAddr 校验并归一化 pprof 监听地址：
// 空 host / 未指定地址（0.0.0.0、::）/ "localhost" 一律收敛到 127.0.0.1；
// 其他非 loopback 地址直接拒绝。
func pprofLoopbackAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid pprof listen address %q: %w", addr, err)
	}
	if port == "" {
		return "", fmt.Errorf("invalid pprof listen address %q: missing port", addr)
	}

	switch strings.ToLower(host) {
	case "", "localhost":
		return net.JoinHostPort("127.0.0.1", port), nil
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return "", fmt.Errorf("refusing to serve pprof on non-loopback host %q", host)
	}
	if ip.IsUnspecified() {
		return net.JoinHostPort("127.0.0.1", port), nil
	}
	if !ip.IsLoopback() {
		return "", fmt.Errorf("refusing to serve pprof on non-loopback address %q", addr)
	}
	return net.JoinHostPort(host, port), nil
}
