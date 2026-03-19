package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/net-agent/remotework/utils"
)

//
// Hub Ping 状态查询（需要同时访问 Networks 和 Services）
//

func (hub *Hub) GetPingState() ([]*PingReport, error) {
	var svcs []*Service
	hub.registry.Range(func(svc *Service) {
		svcs = append(svcs, svc)
	})

	if len(svcs) <= 0 {
		return nil, errors.New("NO SERVICES")
	}

	m := make(map[string]*PingReport)
	for _, svc := range svcs {
		parseDependAndSaveToMap(m, svc)
	}

	reports := []*PingReport{}
	for _, report := range m {
		reports = append(reports, report)

		mnet, err := hub.networks.Find(report.Network)
		if err != nil {
			report.PingResult = err.Error()
			continue
		}
		pingDomain := report.Domain
		if pingDomain == "[server]" {
			pingDomain = ""
		}
		dur, err := mnet.Ping(pingDomain, time.Second*3)
		if err != nil {
			report.PingResult = err.Error()
			continue
		}

		report.PingResult = formatPingDuration(dur)
	}

	return reports, nil
}

func (hub *Hub) GetPingStateString() string {
	reports, err := hub.GetPingState()
	if err != nil {
		return fmt.Sprintf("report ping failed: %v\n", err)
	}

	buf := bytes.NewBufferString("report ping:\n")
	utils.RenderAsciiTable(buf, reports,
		[]string{"index", "network", "domain", "result", "used services"},
		func(d interface{}, index int) []string {
			s := d.(*PingReport)
			return []string{
				fmt.Sprintf("%v", index),
				s.Network,
				s.Domain,
				s.PingResult,
				strings.Join(s.UsedServices, ", "),
			}
		},
	)
	return buf.String()
}

// formatPingDuration 格式化 ping 延迟，最小单位为 ms。
// 低于 1ms 显示为 "<1ms"，低于 1s 显示整数 ms，否则显示保留一位小数的秒。
func formatPingDuration(dur time.Duration) string {
	ms := dur.Milliseconds()
	if ms < 1 {
		return "<1ms"
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", dur.Seconds())
}

//
// 辅助函数
//

func parseDependAndSaveToMap(m map[string]*PingReport, svc *Service) {
	urls := [][]string{
		{"listen", svc.ListenURL},
		{"target", svc.TargetURL},
	}

	for _, u := range urls {
		netname, domain, err := parseURLDepend(u[1])
		if err != nil {
			continue
		}
		if strings.HasPrefix(netname, "tcp") {
			continue
		}
		if _, found := m[netname]; !found {
			m[netname] = &PingReport{
				Network:      netname,
				Domain:       "[server]",
				UsedServices: []string{},
			}
		}
		if domain == "0" || domain == "local" {
			continue
		}

		key := fmt.Sprintf("%v://%v", netname, domain)
		report, found := m[key]
		if !found {
			report = &PingReport{}
			m[key] = report
		}
		report.Network = netname
		report.Domain = domain
		report.UsedServices = append(report.UsedServices, fmt.Sprintf("%v.%v", svc.Name, u[0]))
	}
}

func parseURLDepend(raw string) (string, string, error) {
	if raw == "" {
		return "", "", errors.New("invalid url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	return u.Scheme, u.Hostname(), nil
}
