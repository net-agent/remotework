package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/utils"
)

func handlePingDomain(pingUrl, pingName string, pingTimes int) {
	u, err := url.Parse(pingUrl)
	if err != nil {
		utils.Fatal(syslog, err)
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
		utils.Fatal(syslog, fmt.Sprintf("invalid ping target: '%v'", pingUrl))
	}

	hub := agent.NewHub(nil)
	err = hub.NewAgentNetwork(agent.AgentInfo{
		Name:     "flex",
		Protocol: u.Scheme,
		Address:  u.Host,
		Password: pswd,
		Domain:   pingName,
		WsPath:   wspath,
	})
	if err != nil {
		utils.Fatal(syslog, err)
	}

	mnet, err := hub.FindNetwork("flex")
	if err != nil {
		utils.Fatal(syslog, err)
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
			syslog.Info("ping result", "domain", domain, "err", err)
		} else {
			sum += dur
			total += 1
			if dur > max {
				max = dur
			}
			if dur < min {
				min = dur
			}
			syslog.Info("ping result", "domain", domain, "duration", dur)
		}

		<-time.After(time.Millisecond * 100)
	}
	if total > 0 {
		syslog.Info("ping summary", "max", max, "min", min, "average", time.Duration(int64(sum)/total))
	}
}
