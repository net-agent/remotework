package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/agent"
)

type Monitor struct {
	hub       *agent.NetHub
	listenURL string
	logName   string

	listener net.Listener
}

func NewMonitor(hub *agent.NetHub, listenURL, logName string) *Monitor {
	return &Monitor{
		hub:       hub,
		listenURL: listenURL,
		logName:   logName,
	}
}

func (s *Monitor) Name() string {
	if s.logName != "" {
		return s.logName
	}
	return "monitor"
}

func (s *Monitor) Report() agent.ReportInfo {
	return agent.ReportInfo{
		Name:    s.Name(),
		State:   "uinit",
		Listen:  s.listenURL,
		Target:  "web-ui",
		Actives: 0,
		Dones:   0,
	}
}

func (s *Monitor) Init() error {
	l, err := s.hub.ListenURL(s.listenURL)
	if err != nil {
		return fmt.Errorf("listen url failed: %v", err)
	}
	s.listener = l

	return nil
}

func (s *Monitor) Start() error {
	if s.listener == nil {
		return errors.New("init failed")
	}

	r := mux.NewRouter()
	m := r.PathPrefix("/monitor/api").Subrouter()
	m.Methods("GET").Path("/all-service-report").HandlerFunc(s.HandleAllServiceReport)
	m.Methods("GET").Path("/all-network-report").HandlerFunc(s.HandlerAllNetworkReport)

	return http.Serve(s.listener, nil)
}

func (s *Monitor) HandleAllServiceReport(w http.ResponseWriter, r *http.Request) {
	infos, err := s.hub.ServiceReport()
	s.writejson(w, infos, err)
}

func (s *Monitor) HandlerAllNetworkReport(w http.ResponseWriter, r *http.Request) {
	infos, err := s.hub.NetworkReport()
	s.writejson(w, infos, err)
}

func (s *Monitor) writejson(w http.ResponseWriter, data interface{}, err error) {
	resp := &struct {
		ErrCode int
		ErrMsg  string
		Data    interface{}
	}{}
	if err != nil {
		resp.ErrCode = -1
		resp.ErrMsg = err.Error()
	} else {
		resp.ErrCode = 0
		resp.ErrMsg = ""
		resp.Data = data
	}

	buf, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("json marshal failed: %v", err)))
		return
	}
	_, err = w.Write(buf)
	if err != nil {
		log.Printf("[%v] write repsonse failed: %v\n", s.Name(), err)
	}
}
