package server

import (
	"sync"

	"github.com/net-agent/flex/v2/switcher"
	"github.com/net-agent/mixlisten"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewModuleLogger("server")

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

	// 初始化
	app := switcher.NewServer(config.Server.Password)

	syslog.Info("try to listen", "addr", config.Server.Listen)

	// 监听本地端口（混合协议模式）
	mxl := mixlisten.Listen("tcp", config.Server.Listen)
	mxl.RegisterBuiltIn("flex", "http")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		mxl.Run()
		wg.Done()
	}()

	// 处理Flex协议监听
	flexListener, err := mxl.GetListener("flex")
	if err != nil {
		utils.Fatal(syslog, "get flex listener failed: ", err)
	}
	wg.Add(1)
	go func() {
		ServeTCP(app, config.Server, flexListener)
		wg.Done()
	}()

	// 处理HTTP协议监听
	httpListener, err := mxl.GetListener("http")
	if err != nil {
		utils.Fatal(syslog, "get http listener failed: ", err)
	}
	wg.Add(1)
	go func() {
		ServeWs(app, config.Server, httpListener)
		wg.Done()
	}()

	// 等待所有协成结束
	wg.Wait()
	syslog.Info("server stopped")
}
