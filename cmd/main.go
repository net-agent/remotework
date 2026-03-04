package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/net-agent/remotework/agent"
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
		utils.Fatal(syslog, "load config failed: ", err)
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
