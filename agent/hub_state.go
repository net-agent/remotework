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

func (hub *Hub) GetAllServiceState() ([]ServiceState, error) {
	if len(hub.svcs) <= 0 {
		return nil, errors.New("NO SERVICES")
	}

	var reports []ServiceState
	for _, svc := range hub.svcs {
		reports = append(reports, svc.ServiceState)
	}
	return reports, nil
}

func (hub *Hub) GetAllServiceStateString() string {
	reports, err := hub.GetAllServiceState()
	if err != nil {
		return fmt.Sprintf("report service failed: %v\n", err)
	}

	buf := bytes.NewBufferString("report service:\n")
	utils.RenderAsciiTable(buf, reports,
		[]string{"index", "type", "name", "state", "listen", "target", "actives", "dones"},
		func(d interface{}, index int) []string {
			s := d.(ServiceState)
			return []string{
				fmt.Sprintf("%v", index),
				s.Type,
				s.Name,
				s.State,
				s.ListenURL,
				s.TargetURL,
				fmt.Sprintf("%v", s.Actives),
				fmt.Sprintf("%v", s.Dones),
			}
		},
	)
	return buf.String()
}

func (hub *Hub) GetAllNetworkState() ([]NetworkReport, error) {
	if len(hub.nets) <= 0 {
		return nil, errors.New("NO NETWORKS")
	}

	var reports []NetworkReport
	for _, nt := range hub.nets {
		reports = append(reports, nt.Report())
	}
	return reports, nil
}

func (hub *Hub) GetAllNetworkStateString() string {
	reports, err := hub.GetAllNetworkState()
	if err != nil {
		return fmt.Sprintf("report network failed: %v\n", err)
	}

	buf := bytes.NewBufferString("report network:\n")
	utils.RenderAsciiTable(buf, reports,
		[]string{"index", "name", "addr", "domain", "lsn", "dial"},
		func(d interface{}, index int) []string {
			s := d.(NetworkReport)
			return []string{
				fmt.Sprintf("%v", index),
				s.Name,
				s.Address,
				s.Domain,
				fmt.Sprintf("%v", s.Listens),
				fmt.Sprintf("%v", s.Dials),
			}
		},
	)
	return buf.String()
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

func (hub *Hub) GetPingState() ([]*PingReport, error) {
	// 从service里解析依赖的节点
	if len(hub.svcs) <= 0 {
		return nil, errors.New("NO SERVICES")
	}

	m := make(map[string]*PingReport)
	for _, svc := range hub.svcs {
		parseDependAndSaveToMap(m, svc)
	}

	// 按map里的顺序依次ping
	reports := []*PingReport{}
	for _, report := range m {
		reports = append(reports, report)

		mnet, err := hub.FindNetwork(report.Network)
		if err != nil {
			report.PingResult = err.Error()
			continue
		}
		dur, err := mnet.Ping(report.Domain, time.Second*3)
		if err != nil {
			report.PingResult = err.Error()
			continue
		}

		report.PingResult = fmt.Sprintf("%v", dur)
	}

	return reports, nil
}

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
				Domain:       "",
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