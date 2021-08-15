package main

import (
	"log"
	"path"
	"sync"

	"github.com/net-agent/flex/v2/switcher"
	"github.com/net-agent/mixlisten"
	"github.com/net-agent/remotework/utils"
)

func main() {
	var flags ServerFlags
	flags.Parse()

	// 读取配置
	configName := flags.ConfigFileName
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
			log.Fatal("load config failed: config file not exist!")
		}
	}
	log.Printf("read config from '%v'\n", configName)
	config, err := NewConfig(configName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	// 初始化
	app := switcher.NewServer(config.Server.Password)

	log.Printf("try to listen on '%v'\n", config.Server.Listen)

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
		log.Fatal("get flex listener failed: ", err)
	}
	wg.Add(1)
	go func() {
		ServeTCP(app, config.Server, flexListener)
		wg.Done()
	}()

	// 处理HTTP协议监听
	httpListener, err := mxl.GetListener("http")
	if err != nil {
		log.Fatal("get http listener failed: ", err)
	}
	wg.Add(1)
	go func() {
		ServeWs(app, config.Server, httpListener)
		wg.Done()
	}()

	// 等待所有协成结束
	wg.Wait()
	log.Println("server stopped")
}
