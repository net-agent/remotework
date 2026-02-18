package utils

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof" // 导入 pprof 包以注册路由
)

// PprofServer 管理 pprof HTTP 服务器
type PprofServer struct {
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

// Start 启动 pprof HTTP 服务器
func (p *PprofServer) Start(addr string) error {
	if addr == "" {
		Fatal(p.log, "pprof listen address cannot be empty")
		return nil // 这行不会执行，因为 Fatal 会退出程序
	}

	// 如果服务器已经在运行，先停止它
	if p.server != nil {
		p.Stop()
	}

	p.server = &http.Server{
		Addr: addr,
	}

	go func() {
		p.log.Info("starting pprof server", "addr", addr)
		err := p.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			p.log.Error("pprof server error", "err", err)
		}
	}()

	return nil
}

// Stop 停止 pprof HTTP 服务器
func (p *PprofServer) Stop() {
	if p.server != nil {
		p.server.Close()
		p.server = nil
		p.log.Info("pprof server stopped")
	}
}

// IsRunning 检查 pprof 服务器是否正在运行
func (p *PprofServer) IsRunning() bool {
	return p.server != nil
}
