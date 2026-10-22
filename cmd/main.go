package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
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
		if err := runAgent(flags.ConfigFileName); err != nil {
			syslog.Error("agent exited with error", "err", err)
			os.Exit(1)
		}
	case "server":
		if err := server.RunServer(flags.ConfigFileName); err != nil {
			syslog.Error("server exited with error", "err", err)
			os.Exit(1)
		}
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

func runAgent(configFile string) error {
	token := os.Getenv("REMOTEWORK_API_TOKEN")

	resolved, err := utils.ResolveConfigFile(configFile)
	if err != nil {
		fatalExit("config", 1, err)
	}
	syslog.Info("read config", "path", resolved)

	config, cfgErrs := configv2.LoadAndCheck(resolved)
	if config == nil {
		// 配置整体不可解析：错误文本可能含畸形 URL 中的凭证，先脱敏再展示/上报
		configError := utils.RedactText(cfgErrs.Error())
		syslog.Error("config parse failed, starting in degraded mode", "err", configError)
		return runDegradedAgent(resolved, token, configError)
	}
	// 坏项（link/tunnel）已被 LoadAndCheck 丢弃；逐项给出原因，好项照常加载
	for _, e := range cfgErrs.Errors {
		syslog.Error("config item invalid, skipped", "err", utils.RedactText(e.Error()))
	}

	if managedMode && !config.API.Enable {
		// 托管模式依赖 stdout 的 ready 控制行；API 关闭时进程无法完成握手，
		// 宿主只能等 5s 超时。启动前直接报配置错误，给出明确 stage/error。
		fatalExit("config", 1, errors.New("api.enable must be true when -log-format=json (managed mode)"))
	}

	if config.Pprof.Enable {
		pp := utils.NewPprofServer(syslog.With("module", "pprof"))
		if err := pp.Start(config.Pprof.Listen); err != nil {
			syslog.Error("pprof server start failed", "err", err)
		} else {
			defer pp.Stop()
		}
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

	// 关停预算覆盖最坏路径：Drain(10s) + 两条 cleanup(2×5s) + API(5s)。
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	err = host.Shutdown(ctx)
	if err != nil {
		syslog.Error("shutdown incomplete", "err", err)
		return fmt.Errorf("shutdown: %w", err)
	}
	syslog.Info("main process exit")
	return nil
}

// runDegradedAgent 配置损坏时启动降级模式：只运行 API 插件，便于修复配置
func runDegradedAgent(configPath, token, configError string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err := host.Shutdown(ctx); err != nil {
		syslog.Error("degraded shutdown incomplete", "err", err)
		return fmt.Errorf("shutdown: %w", err)
	}
	syslog.Info("degraded agent exit")
	return nil
}

// stopTrigger 返回停止触发器与等待器：OS 信号或 API /stop 请求均可触发。
// 第一次信号触发优雅关闭；等待器返回后会监听第二次信号并立即退出，
// 避免关停卡住时只能 SIGKILL。
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
			go func() {
				sig := <-sigCh
				syslog.Warn("second signal received, forcing exit", "signal", sig)
				os.Exit(130)
			}()
		}
}

func validateConfig(configFile string) {
	resolved, err := utils.ResolveConfigFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve config: %v\n", err)
		os.Exit(1)
	}
	_, cfgErrs := configv2.LoadAndCheck(resolved)
	if len(cfgErrs.Errors) == 0 {
		fmt.Println("OK")
		return
	}
	// 同一 -mode validate 也支持 relay 配置：agent 配置严格解码会因
	// 顶层 "server" 字段失败，此时回退到 server 配置校验。
	if _, serr := server.NewConfig(resolved); serr == nil {
		fmt.Println("OK")
		return
	} else if strings.Contains(cfgErrs.Error(), `unknown field "server"`) {
		// 明确是 relay 配置：展示 server 校验的具体错误，而不是 agent schema 的报错。
		fmt.Fprintf(os.Stderr, "%s\n", utils.RedactText(serr.Error()))
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "%s\n", utils.RedactText(cfgErrs.Error()))
	os.Exit(1)
}
