package server

import (
	"path"
	"sync"

	"github.com/net-agent/flex/v2/switcher"
	"github.com/net-agent/mixlisten"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewNamedLogger("sys", false)

func RunServer(configName string) {
	if !utils.FileExist(configName) {
		// try `config.json` or `config.toml`
		dir := path.Dir(configName)
		configJson := path.Join(dir, "config.json")
		configToml := path.Join(dir, "config.toml")
		if utils.FileExist(configJson) {
			configName = configJson
		} else if utils.FileExist(configToml) {
			configName = configToml
		} else {
			syslog.Fatal("load config failed: config file not exist!")
		}
	}
	syslog.Printf("read config from '%v'\n", configName)
	config, err := NewConfig(configName)
	if err != nil {
		syslog.Fatal("load config failed: ", err)
	}

	// 初始化
	app := switcher.NewServer(config.Server.Password)

	syslog.Printf("try to listen on '%v'\n", config.Server.Listen)

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
		syslog.Fatal("get flex listener failed: ", err)
	}
	wg.Add(1)
	go func() {
		ServeTCP(app, config.Server, flexListener)
		wg.Done()
	}()

	// 处理HTTP协议监听
	httpListener, err := mxl.GetListener("http")
	if err != nil {
		syslog.Fatal("get http listener failed: ", err)
	}
	wg.Add(1)
	go func() {
		ServeWs(app, config.Server, httpListener)
		wg.Done()
	}()

	// 等待所有协成结束
	wg.Wait()
	syslog.Println("server stopped")
}
