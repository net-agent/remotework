package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/net-agent/flex/v3/stream"
	"github.com/net-agent/remotework/utils"
)

//
// NetworkRegistry 状态报告方法
//

func (nr *NetworkRegistry) GetAllState() ([]NetworkReport, error) {
	nr.mut.RLock()
	defer nr.mut.RUnlock()

	if len(nr.nets) <= 0 {
		return nil, errors.New("NO NETWORKS")
	}

	var reports []NetworkReport
	for _, nt := range nr.nets {
		reports = append(reports, nt.Report())
	}
	return reports, nil
}

func (nr *NetworkRegistry) GetAllStateString() string {
	reports, err := nr.GetAllState()
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

// streamStateProvider 用于获取数据流状态的可选接口
type streamStateProvider interface {
	GetStreamStates() (actives, closeds []*stream.State)
}

func getDataStreamStateByNetwork(mnet Network) (actives, closeds []*stream.State) {
	// 穿透装饰器访问内部 Network
	inner := mnet
	if rn, ok := mnet.(*reportingNetwork); ok {
		inner = rn.Network
	}
	provider, ok := inner.(streamStateProvider)
	if !ok {
		return nil, nil
	}
	return provider.GetStreamStates()
}

type DataStreamState struct {
	Network string
	Actives []*stream.State
	Closeds []*stream.State
}

func (nr *NetworkRegistry) GetAllDataStreamStateString() string {
	buf := bytes.NewBufferString("report actived stream:\n")

	nr.mut.RLock()
	nets := make(map[string]*reportingNetwork, len(nr.nets))
	for k, v := range nr.nets {
		nets[k] = v
	}
	nr.mut.RUnlock()

	for networkName, rn := range nets {
		states, _ := getDataStreamStateByNetwork(rn)
		if len(states) > 0 {
			utils.RenderAsciiTable(buf, states,
				[]string{"index", "network", "local", "remote", "readed", "wrote", "alive"},
				func(d interface{}, index int) []string {
					st := d.(*stream.State)
					alived := time.Since(st.Created)
					if st.IsClosed {
						alived = st.Closed.Sub(st.Created)
					}
					return []string{
						fmt.Sprint(index),
						networkName,
						fmt.Sprintf("%v(%v)", st.LocalDomain, st.LocalAddr.String()),
						fmt.Sprintf("%v(%v)", st.RemoteDomain, st.RemoteAddr.String()),
						fmt.Sprint(st.ConnReadSize),
						fmt.Sprint(st.ConnWriteSize),
						fmt.Sprint(alived),
					}
				},
			)
		}
	}
	return buf.String()
}

func (nr *NetworkRegistry) GetDataStreamState(limits int, networks ...string) []*DataStreamState {
	resp := []*DataStreamState{}
	for _, network := range networks {
		mnet, err := nr.Find(network)
		if err != nil {
			resp = append(resp, nil)
			continue
		}

		actives, closeds := getDataStreamStateByNetwork(mnet)
		size := len(closeds)
		if size > limits {
			closeds = closeds[size-limits : size]
		}
		resp = append(resp, &DataStreamState{network, actives, closeds})
	}

	return resp
}

//
// Hub 跨域状态报告方法（需要同时访问 Networks 和 Services）
//

func (hub *Hub) GetPingState() ([]*PingReport, error) {
	var svcs []*Service
	hub.services.Range(func(svc *Service) {
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
		dur, err := mnet.Ping(report.Domain, time.Second*3)
		if err != nil {
			report.PingResult = err.Error()
			continue
		}

		report.PingResult = fmt.Sprintf("%v", dur)
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

//
// Hub 委托方法（保持外部 API 兼容）
//

func (hub *Hub) GetAllServiceState() ([]ServiceState, error)  { return hub.services.GetAllState() }
func (hub *Hub) GetAllServiceStateString() string             { return hub.services.GetAllStateString() }
func (hub *Hub) GetAllNetworkState() ([]NetworkReport, error) { return hub.networks.GetAllState() }
func (hub *Hub) GetAllNetworkStateString() string             { return hub.networks.GetAllStateString() }
func (hub *Hub) GetAllDataStreamStateString() string {
	return hub.networks.GetAllDataStreamStateString()
}
func (hub *Hub) GetDataStreamState(limits int, networks ...string) []*DataStreamState {
	return hub.networks.GetDataStreamState(limits, networks...)
}
func (hub *Hub) PingDomain(network, domain string) (time.Duration, error) {
	return hub.networks.PingDomain(network, domain)
}

//
// 保留的包级辅助函数（不变）
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
