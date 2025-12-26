package utils

import (
	"net/http"
	_ "net/http/pprof" // 导入 pprof 包以注册路由
)

// PprofServer 管理 pprof HTTP 服务器
type PprofServer struct {
	server *http.Server
	logger *NamedLogger
}

// NewPprofServer 创建新的 pprof 服务器实例
func NewPprofServer(logger *NamedLogger) *PprofServer {
	return &PprofServer{
		logger: logger,
	}
}

// Start 启动 pprof HTTP 服务器
func (p *PprofServer) Start(addr string) error {
	if addr == "" {
		p.logger.Fatal("pprof listen address cannot be empty")
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
		p.logger.Printf("Starting pprof server on http://%s", addr)
		err := p.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			p.logger.Printf("pprof server error: %v", err)
		}
	}()

	return nil
}

// Stop 停止 pprof HTTP 服务器
func (p *PprofServer) Stop() {
	if p.server != nil {
		p.server.Close()
		p.server = nil
		p.logger.Println("pprof server stopped")
	}
}

// IsRunning 检查 pprof 服务器是否正在运行
func (p *PprofServer) IsRunning() bool {
	return p.server != nil
}
