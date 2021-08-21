package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/agent/service"
	"github.com/net-agent/remotework/utils"
)

func loadConfig() *agent.Config {
	var flags agent.AgentFlags
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
	config, err := agent.NewConfig(configName)
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

	var wg sync.WaitGroup
	hub := agent.NewNetHub()

	if len(config.Agents) <= 0 {
		if config.Agent.Network == "" {
			config.Agent.Enable = true
			config.Agent.Network = "flex"
		}
		config.Agents = append(config.Agents, config.Agent)
	}

	log.Println("startup agents:")
	networkCount := 0
	for index, info := range config.Agents {
		var agt agent.AgentInfo = info // copy value
		if !agt.Enable {
			log.Printf("agents[%v] disabled. network='%v' domain='%v'\n",
				index, agent.Green(agt.Network), agent.Green(agt.Domain))
			continue
		}

		log.Printf("agents[%v] connect to network='%v' domain='%v'\n",
			index, agent.Green(agt.Network), agent.Green(agt.Domain))

		mnet := agent.NewNetwork(agt.GetConnectFn())
		ch := make(chan struct{}, 4)
		go mnet.KeepAlive(ch)
		<-ch

		if agt.QuickTrust.Enable {
			svc := service.NewQuickTrust(&agt, mnet)
			svc.Start(&wg)
		}

		err := hub.AddNetwork(agt.Network, mnet)
		if err != nil {
			log.Printf("add network to hub failed: %v\n", err)
		} else {
			networkCount++
		}
	}
	log.Printf("%v agents added to hub\n\n", networkCount)
	if networkCount == 0 {
		return
	}

	log.Println("startup services:")
	log.Println("-------------------------------------------------------------------------")
	log.Println("  # command        type                   listen                   target")
	log.Println("-------------------------------------------------------------------------")

	// 初始化services
	svcs := []service.Service{}
	for i, info := range config.Services {
		info.SetIndex(i)
		svc := service.NewService(hub, info)
		if svc != nil {
			svcs = append(svcs, svc)
			log.Printf("%3v %7v %v\n", i, "run", svc.Info())
		} else {
			log.Printf("%3v %7v %v (unknown service type)\n", i, "-", info.Type)
		}
	}

	log.Println("-------------------------------------------------------------------------")

	// 开启服务
	for i, svc := range svcs {
		err := svc.Start(&wg)
		if err != nil {
			log.Printf("[runsvc] service start failed. svcindex=%v err=%v\n", i, err)
		}
	}
	wg.Wait()

	log.Println("main process exit.")
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
