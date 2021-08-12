package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/agent/service"
)

func loadConfig() *agent.Config {
	var flags agent.AgentFlags
	flags.Parse()

	// 读取配置
	log.Printf("> read config from '%v'\n", flags.ConfigFileName)
	var err error
	config, err := agent.NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	return config
}

func main() {
	config := loadConfig()

	// 初始化日志文件
	logoutput, shouldClose := initLogOutput()
	if shouldClose && logoutput != nil {
		defer logoutput.Close()
	}

	log.Printf("domain='%v'\n", agent.Green(config.Agent.Domain))

	mnet := agent.NewNetwork(config.GetConnectFn())
	ch := make(chan struct{}, 4)
	go mnet.KeepAlive(ch)

	<-ch

	log.Println("startup services:")
	log.Println("-------------------------------------------------------------------------")
	log.Println("  # command        type                   listen                   target")
	log.Println("-------------------------------------------------------------------------")

	// 初始化services
	svcs := []service.Service{}
	for i, info := range config.Services {
		info.SetIndex(i)
		svc := service.NewService(mnet, info)
		if svc != nil {
			svcs = append(svcs, svc)
			log.Printf("%3v %7v %v\n", i, "run", svc.Info())
		} else {
			log.Printf("%3v %7v %v (unknown service type)\n", i, "-", info.Type)
		}
	}

	log.Println("-------------------------------------------------------------------------")

	// 开启服务
	var wg sync.WaitGroup
	for i, svc := range svcs {
		err := svc.Start(&wg)
		if err != nil {
			log.Printf("[runsvc] service start failed. svcindex=%v err=%v\n", i, err)
		}
	}
	wg.Wait()
}

func initLogOutput() (f *os.File, shouldClose bool) {
	if FileExist("./temp") {
		fpath := fmt.Sprintf("./temp/agent_%v.log", time.Now().Format("20060102_150405"))
		fmt.Printf("write log to file: %v\n", fpath)
		f, err := os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(f)
			return f, true
		}
	}

	log.SetOutput(os.Stdout)
	return os.Stdout, false
}

func FileExist(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
