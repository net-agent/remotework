package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/plugins/api"
	"github.com/net-agent/remotework/plugins/netflex"
	"github.com/net-agent/remotework/plugins/nettcp"
	"github.com/net-agent/remotework/plugins/tunnel"
	"github.com/net-agent/remotework/server"
	"github.com/net-agent/remotework/utils"
)

var syslog = utils.NewModuleLogger("sys")

func main() {
	var flags ClientFlags
	flags.Parse()

	switch flags.RunMode {
	case "agent":
		runAgent(flags.ConfigFileName)
	case "server":
		server.RunServer(flags.ConfigFileName)
	case "validate":
		validateConfig(flags.ConfigFileName)
	default:
		utils.Fatal(syslog, "invalid run-mode: ", flags.RunMode)
	}
}

// loadPlugin 装载插件并记录失败（不中断其余插件）
func loadPlugin(host *kernel.Host, p kernel.Plugin) {
	if err := host.Load(p); err != nil {
		syslog.Warn("load plugin failed", "name", p.Info().Name, "err", err)
	}
}

func runAgent(configFile string) {
	resolved, err := utils.ResolveConfigFile(configFile)
	if err != nil {
		utils.Fatal(syslog, "load config failed: ", err)
	}
	syslog.Info("read config", "path", resolved)

	config, cfgErrs := configv2.LoadAndCheck(resolved)
	if config == nil {
		syslog.Error("config parse failed, starting in degraded mode", "err", cfgErrs)
		runDegradedAgent(resolved)
		return
	}
	// 坏项（link/tunnel）跳过并逐个给出原因，好项照常加载——单项失败互不影响
	for _, e := range cfgErrs.Errors {
		syslog.Error("config item invalid, skipped", "err", e.Error())
	}

	if config.Pprof.Enable {
		pp := utils.NewPprofServer(syslog.With("module", "pprof"))
		pp.Start(config.Pprof.Listen)
		defer pp.Stop()
	}

	host := kernel.NewHost(nil, nil)
	requestStop, waitStop := stopTrigger()

	// 内置本地网络
	for _, alias := range []string{"tcp", "tcp4", "tcp6"} {
		loadPlugin(host, nettcp.New(alias))
	}

	// link 网络插件
	for alias, link := range config.Links {
		loadPlugin(host, netflex.New(alias, link))
	}

	// tunnel 服务插件
	for _, t := range config.Tunnels {
		loadPlugin(host, tunnel.New(t))
	}

	if config.API.Enable {
		loadPlugin(host, apiplugin.New(config.API, requestStop))
	}

	waitStop()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := host.Shutdown(ctx); err != nil {
		syslog.Warn("shutdown incomplete", "err", err)
	}
	syslog.Info("main process exit")
}

// runDegradedAgent 配置损坏时启动降级模式：只运行 API 插件，便于修复配置
func runDegradedAgent(configPath string) {
	apiInfo := configv2.APIInfo{Enable: true, Listen: "127.0.0.1:8080"}
	if raw, err := configv2.LoadFile(configPath); err == nil {
		if raw.API.Enable {
			apiInfo = raw.API
		}
	}

	if !apiInfo.Enable {
		syslog.Error("API is disabled in config, cannot start degraded mode")
		os.Exit(1)
	}

	host := kernel.NewHost(nil, nil)
	requestStop, waitStop := stopTrigger()

	for _, alias := range []string{"tcp", "tcp4", "tcp6"} {
		loadPlugin(host, nettcp.New(alias))
	}
	loadPlugin(host, apiplugin.New(apiInfo, requestStop))

	syslog.Warn("running in degraded mode — no networks or services loaded")

	waitStop()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	host.Shutdown(ctx)
	syslog.Info("degraded agent exit")
}

// stopTrigger 返回停止触发器与等待器：OS 信号或 API /stop 请求均可触发
func stopTrigger() (requestStop func(), waitStop func()) {
	stopCh := make(chan struct{})
	var once sync.Once

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	return func() { once.Do(func() { close(stopCh) }) },
		func() {
			select {
			case sig := <-sigCh:
				syslog.Info("close with signal", "signal", sig)
			case <-stopCh:
				syslog.Info("close with stop request")
			}
		}
}

func validateConfig(configFile string) {
	resolved, err := utils.ResolveConfigFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve config: %v\n", err)
		os.Exit(1)
	}
	_, cfgErrs := configv2.LoadAndCheck(resolved)
	if len(cfgErrs.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "%v\n", cfgErrs)
		os.Exit(1)
	}
	fmt.Println("OK")
}
