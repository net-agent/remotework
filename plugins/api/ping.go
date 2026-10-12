package apiplugin

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/net-agent/remotework/kernel"
	"github.com/net-agent/remotework/vnet"
	"github.com/net-agent/remotework/vservice"
)

// GetPingState 汇总各隧道依赖的网络/domain 并逐一探测
func (b *backend) GetPingState() ([]*vnet.PingReport, error) {
	var svcs []*vservice.Service
	for _, ps := range b.catalog.List() {
		if ps.Info.Kind != "tunnel" {
			continue
		}
		name := strings.TrimPrefix(ps.Info.Name, tunnelNamePrefix)
		svc, ok := kernel.Lookup[*vservice.Service](b.ctx, name)
		if ok {
			svcs = append(svcs, svc)
		}
	}

	if len(svcs) <= 0 {
		return nil, errors.New("NO SERVICES")
	}

	m := make(map[string]*vnet.PingReport)
	for _, svc := range svcs {
		parseDependAndSaveToMap(m, svc)
	}

	reports := []*vnet.PingReport{}
	for _, report := range m {
		reports = append(reports, report)

		mnet, ok := kernel.Lookup[vnet.Network](b.ctx, report.Network)
		if !ok {
			report.PingResult = fmt.Sprintf("network='%v' not found", report.Network)
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

func parseDependAndSaveToMap(m map[string]*vnet.PingReport, svc *vservice.Service) {
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
			m[netname] = &vnet.PingReport{
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
			report = &vnet.PingReport{}
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
