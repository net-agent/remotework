package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/agent/configv2"
	"github.com/net-agent/remotework/api"
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

func runAgent(configFile string) {
	resolved, err := utils.ResolveConfigFile(configFile)
	if err != nil {
		utils.Fatal(syslog, "load config failed: ", err)
	}
	syslog.Info("read config", "path", resolved)

	config, err := agent.NewCanonicalConfig(resolved)
	if err != nil {
		syslog.Error("config has errors, starting in degraded mode", "err", err)
		runDegradedAgent(resolved)
		return
	}

	if config.Pprof.Enable {
		pp := utils.NewPprofServer(syslog.With("module", "pprof"))
		pp.Start(config.Pprof.Listen)
		defer pp.Stop()
	}

	hub := agent.NewHub(nil)
	if err := hub.MountConfig(config); err != nil {
		syslog.Warn("mount config warning", "err", err)
	}

	if config.API.Enable {
		apiSrv := api.New(hub, config.API, syslog.With("module", "api"))
		apiSrv.Start()
		defer apiSrv.Stop()
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		sig := <-ch
		syslog.Info("close with signal", "signal", sig)
		hub.Stop()
	}()

	hub.Start()
	syslog.Info("main process exit")
}

// runDegradedAgent starts the agent with no networks/services but with the API
// server running, so the UI can still connect and the user can fix the config.
func runDegradedAgent(configPath string) {
	// Try to extract API config from the raw file (even if normalization failed)
	apiInfo := agent.APIInfo{Enable: true, Listen: "127.0.0.1:8080"}
	if raw, err := configv2.LoadFile(configPath); err == nil {
		if raw.API.Enable {
			apiInfo = raw.API
		}
	}

	if !apiInfo.Enable {
		syslog.Error("API is disabled in config, cannot start degraded mode")
		os.Exit(1)
	}

	hub := agent.NewHub(nil)

	apiSrv := api.New(hub, apiInfo, syslog.With("module", "api"))
	apiSrv.Start()
	defer apiSrv.Stop()

	syslog.Warn("running in degraded mode — no networks or services loaded")

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		sig := <-ch
		syslog.Info("close with signal", "signal", sig)
		hub.Stop()
	}()

	hub.Start()
	syslog.Info("degraded agent exit")
}

func validateConfig(configFile string) {
	resolved, err := utils.ResolveConfigFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve config: %v\n", err)
		os.Exit(1)
	}
	if _, err := agent.NewCanonicalConfig(resolved); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK")
}
