package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/net-agent/remotework/api"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/kernel"
	apiplugin "github.com/net-agent/remotework/plugins/api"
	"github.com/net-agent/remotework/plugins/netflex"
	"github.com/net-agent/remotework/plugins/nettcp"
	"github.com/net-agent/remotework/plugins/tunnel"
	"github.com/net-agent/remotework/server"
	"github.com/net-agent/remotework/utils"
)

// syslog 在 main() 中 SetFormat 之后初始化：
// FanoutHandler.WithAttrs 是快照拷贝，init 期创建的 logger 会一直沿用
// 初始的 stderr text handler，必须在日志格式确定后再绑定。
var syslog *slog.Logger

func main() {
	var flags ClientFlags
	flags.Parse()

	managedMode = flags.LogFormat == "json"
	utils.SetFormat(flags.LogFormat)
	syslog = utils.NewModuleLogger("sys")

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

// makeAPIOptions 组装进程层注入给 api 插件的运行时选项
func makeAPIOptions(token, mode, configPath, configError string) apiplugin.Options {
	return apiplugin.Options{
		Token: token,
		Meta: api.Meta{
			Protocol:    protocolVersion,
			Version:     version,
			Commit:      commit,
			Mode:        mode,
			ConfigPath:  configPath,
			ConfigError: configError,
		},
	}
}

// startAPIPlugin 装载 api 插件并按 C3 协议完成启动握手：
// 轮询插件状态直至 Running（输出 ready 行）/ Failed / 超时（fatal 行 + 退出码 2）。
// api 插件只依赖内核预置能力，不会长期 Pending。
func startAPIPlugin(host *kernel.Host, cfg configv2.APIInfo, requestStop func(), opts apiplugin.Options) {
	apip := apiplugin.New(cfg, requestStop, opts)
	if err := host.Load(apip); err != nil {
		fatalExit("bind", 2, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		st, err := host.State("api")
		if err == nil {
			switch st.State {
			case kernel.StateRunning:
				printReady(apip.Addr(), opts.Meta.Mode, opts.Meta.ConfigError)
				return
			case kernel.StateFailed:
				fatalExit("bind", 2, errors.New(st.LastErr))
			}
		}
		if time.Now().After(deadline) {
			fatalExit("bind", 2, errors.New("timeout waiting for api plugin to start"))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func runAgent(configFile string) {
	token := os.Getenv("REMOTEWORK_API_TOKEN")

	resolved, err := utils.ResolveConfigFile(configFile)
	if err != nil {
		fatalExit("config", 1, err)
	}
	syslog.Info("read config", "path", resolved)

	config, cfgErrs := configv2.LoadAndCheck(resolved)
	if config == nil {
		syslog.Error("config parse failed, starting in degraded mode", "err", cfgErrs)
		runDegradedAgent(resolved, token, cfgErrs.Error())
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
	startParentWatchdog(requestStop, syslog)

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
		startAPIPlugin(host, config.API, requestStop, makeAPIOptions(token, "normal", resolved, ""))
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
func runDegradedAgent(configPath, token, configError string) {
	apiInfo := configv2.APIInfo{Enable: true, Listen: "127.0.0.1:8080"}
	if raw, err := configv2.LoadFile(configPath); err == nil {
		if raw.API.Enable {
			apiInfo = raw.API
		}
	}

	if !apiInfo.Enable {
		fatalExit("config", 1, errors.New("config parse failed and api.enable=false, cannot start degraded mode"))
	}

	host := kernel.NewHost(nil, nil)
	requestStop, waitStop := stopTrigger()
	startParentWatchdog(requestStop, syslog)

	for _, alias := range []string{"tcp", "tcp4", "tcp6"} {
		loadPlugin(host, nettcp.New(alias))
	}
	startAPIPlugin(host, apiInfo, requestStop, makeAPIOptions(token, "degraded", configPath, configError))

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
