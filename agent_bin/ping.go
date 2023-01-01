package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/net-agent/remotework/agent"
)

func handlePingDomain(pingUrl, pingName string) {
	u, err := url.Parse(pingUrl)
	if err != nil {
		syslog.Fatal(err)
	}
	pswd, _ := u.User.Password()
	wspath := ""
	if strings.HasPrefix(u.Scheme, "ws") {
		wspath = u.Path
	}
	if pingName == "" {
		hostname, _ := os.Hostname()
		pingName = fmt.Sprintf("pingclient_%v", hostname)
	}
	domain := u.User.Username()

	if u.Scheme == "" || u.Host == "" || pswd == "" || domain == "" {
		syslog.Fatal(fmt.Sprintf("invalid ping target: '%v'", pingUrl))
	}

	hub := agent.NewHub()
	err = hub.AddNetwork(agent.NewNetwork(hub, agent.AgentInfo{
		Name:     "flex",
		Protocol: u.Scheme,
		Address:  u.Host,
		Password: pswd,
		Domain:   pingName,
		WsPath:   wspath,
	}))
	if err != nil {
		syslog.Fatal(err)
	}

	mnet, err := hub.FindNetwork("flex")
	if err != nil {
		syslog.Fatal(err)
	}

	for i := 0; i < 4; i++ {
		dur, err := mnet.Ping(domain, time.Second*3)
		if err != nil {
			syslog.Printf("ping '%v': %v\n", domain, err)
			continue
		}

		syslog.Printf("ping '%v': %v\n", domain, dur)
	}
}
