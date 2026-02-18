package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/net-agent/remotework/agent"
)

func waitCloseSignal(hub *agent.Hub) {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM)

	sig := <-ch
	syslog.Info("close with signal", "signal", sig)
	releaseSysTray()
	hub.Stop()
}
