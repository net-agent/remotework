package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/net-agent/remotework/agent"
)

func handlePingDomain(pingUrl, pingName string, pingTimes int) {
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
		Enable:   true,
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

	if pingTimes <= 0 {
		pingTimes = 8
	}

	var max = time.Second * 0
	var min = time.Second * 9999
	var sum = time.Second * 0
	var total = int64(0)
	for i := 0; i < pingTimes; i++ {
		dur, err := mnet.Ping(domain, time.Second*3)
		if err != nil {
			syslog.Printf("ping '%v': %v\n", domain, err)
		} else {
			sum += dur
			total += 1
			if dur > max {
				max = dur
			}
			if dur < min {
				min = dur
			}
			syslog.Printf("ping '%v': %v\n", domain, dur)
		}

		<-time.After(time.Millisecond * 100)
	}
	if total > 0 {
		syslog.Printf("MAX: %v, MIN: %v, AVERAGE: %v\n", max, min, time.Duration(int64(sum)/total))
	}
}
