package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/net-agent/remotework/agent"
)

func waitCloseSignal(hub *agent.Hub) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM)

	sig := <-ch
	syslog.Printf("close with signal=%v\n", sig)
	releaseSysTray()
	hub.StopServices()
}
