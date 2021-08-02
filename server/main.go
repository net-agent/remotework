package main

import (
	"log"
	"sync"

	"github.com/net-agent/flex/switcher"
	"github.com/net-agent/mixlisten"
)

func main() {
	var flags ServerFlags
	flags.Parse()

	// 读取配置
	log.Printf("read config from '%v'\n", flags.ConfigFileName)
	config, err := NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	// 初始化
	app := switcher.NewServer()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		app.Run()
		wg.Done()
	}()

	log.Printf("try to listen on '%v'\n", config.Server.Listen)

	// 监听本地端口（混合协议模式）
	mxl := mixlisten.Listen("tcp", config.Server.Listen)
	mxl.Register(mixlisten.Flex())
	mxl.Register(mixlisten.HTTP())
	wg.Add(1)
	go func() {
		mxl.Run()
		wg.Done()
	}()

	// 处理Flex协议监听
	flexListener, err := mxl.GetListener(mixlisten.Flex().Name())
	if err != nil {
		log.Fatal("get flex listener failed: ", err)
	}
	wg.Add(1)
	go func() {
		ServeTCP(app, config.Server, flexListener)
		wg.Done()
	}()

	// 处理HTTP协议监听
	httpListener, err := mxl.GetListener(mixlisten.HTTP().Name())
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
