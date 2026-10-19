package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/utils"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, nil, statusDTO{
		Running:     s.backend.IsRunning(),
		Mode:        s.meta.Mode,
		Version:     s.meta.Version,
		Commit:      s.meta.Commit,
		Protocol:    s.meta.Protocol,
		UptimeMs:    time.Since(s.startedAt).Milliseconds(),
		ConfigPath:  s.meta.ConfigPath,
		ConfigError: s.meta.ConfigError,
	})
}

func (s *Server) handleNetworks(w http.ResponseWriter, r *http.Request) {
	reports, err := s.backend.GetAllNetworkState()
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	dtos := make([]NetworkStateDTO, 0, len(reports))
	for _, r := range reports {
		if r.Name == "tcp4" || r.Name == "tcp6" {
			continue
		}
		dtos = append(dtos, toNetworkDTO(r))
	}
	utils.WriteJSON(w, nil, dtos)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	states, err := s.backend.GetAllServiceState()
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	dtos := make([]ServiceStateDTO, 0, len(states))
	for _, st := range states {
		dtos = append(dtos, toServiceDTO(st))
	}
	utils.WriteJSON(w, nil, dtos)
}

func (s *Server) handleListeningPorts(w http.ResponseWriter, r *http.Request) {
	ports, err := utils.ListListeningPorts()
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	dtos := make([]ListeningPortDTO, 0, len(ports))
	for _, port := range ports {
		dtos = append(dtos, ListeningPortDTO{
			Port:        port.Port,
			Protocol:    port.Protocol,
			Address:     port.Address,
			PID:         port.PID,
			ProcessName: port.ProcessName,
			ProcStartMs: port.ProcStartMs,
		})
	}
	utils.WriteJSON(w, nil, dtos)
}

func (s *Server) handleStreams(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	filterNetwork := r.URL.Query().Get("network")

	reports, err := s.backend.GetAllNetworkState()
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	var networks []string
	for _, r := range reports {
		if filterNetwork == "" || r.Name == filterNetwork {
			networks = append(networks, r.Name)
		}
	}

	streamStates := s.backend.GetDataStreamState(limit, networks...)

	now := time.Now()
	var dtos []StreamStateDTO
	for _, ds := range streamStates {
		if ds == nil {
			continue
		}
		// 活跃流采样实时速率，并标注归属服务
		rates := s.meter.sample(ds.Network, ds.Actives, now)
		for _, st := range ds.Actives {
			dto := toStreamDTO(ds.Network, st)
			if rate, ok := rates[st.Index]; ok {
				dto.ReadBps = rate[0]
				dto.WriteBps = rate[1]
			}
			dto.ServiceName = s.backend.GetStreamService(ds.Network, int64(st.Index))
			dtos = append(dtos, dto)
		}
		for _, st := range ds.Closeds {
			dto := toStreamDTO(ds.Network, st)
			dto.ServiceName = s.backend.GetStreamService(ds.Network, int64(st.Index))
			dtos = append(dtos, dto)
		}
	}
	if dtos == nil {
		dtos = []StreamStateDTO{}
	}
	utils.WriteJSON(w, nil, dtos)
}

// handleCloseStream 手动关闭指定网络中的一条流
func (s *Server) handleCloseStream(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	network := vars["network"]

	id, err := strconv.ParseInt(vars["id"], 10, 32)
	if err != nil {
		utils.WriteJSON(w, fmt.Errorf("invalid stream id: '%v'", vars["id"]), nil)
		return
	}

	if err := s.backend.CloseStream(network, int32(id)); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	utils.WriteJSON(w, nil, map[string]string{"status": "ok"})
}

func (s *Server) handlePingAll(w http.ResponseWriter, r *http.Request) {
	reports, err := s.backend.GetPingState()
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	dtos := make([]PingResultDTO, 0, len(reports))
	for _, r := range reports {
		dtos = append(dtos, toPingDTO(r))
	}
	utils.WriteJSON(w, nil, dtos)
}

func (s *Server) handlePingSingle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	network := vars["network"]
	domain := vars["domain"]

	dur, err := s.backend.PingDomain(network, domain)
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	utils.WriteJSON(w, nil, map[string]interface{}{
		"network": network,
		"domain":  domain,
		"result":  dur.String(),
	})
}

func (s *Server) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level string `json:"level"`
	}
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	var level slog.Level
	switch req.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		utils.WriteJSON(w, errors.New("invalid level, use: debug/info/warn/error"), nil)
		return
	}

	utils.SetLevel(level)
	s.log.Info("log level changed", "level", req.Level)
	utils.WriteJSON(w, nil, map[string]string{"level": req.Level})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, nil, map[string]string{"status": "stopping"})
	go s.backend.RequestStop()
}

// handleAddLink 运行时动态挂载 link（网络）
func (s *Server) handleAddLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Alias string `json:"alias"`
		URL   string `json:"url"`
	}
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	if req.Alias == "" || req.URL == "" {
		utils.WriteJSON(w, errors.New("alias and url are required"), nil)
		return
	}

	// Normalize the link
	raw := &configv2.RawConfig{
		Links: map[string]string{req.Alias: req.URL},
	}
	canonical, err := configv2.Normalize(raw)
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	link := canonical.Links[req.Alias]
	if link.As == "" {
		utils.WriteJSON(w, errors.New("'as' parameter is required in link URL"), nil)
		return
	}

	if err := s.backend.MountLink(req.Alias, link); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	utils.WriteJSON(w, nil, map[string]string{"status": "ok", "alias": req.Alias})
}

// handleAddTunnel 运行时动态挂载 tunnel（服务）
func (s *Server) handleAddTunnel(w http.ResponseWriter, r *http.Request) {
	var tunnelRaw configv2.TunnelRaw
	if err := utils.ReadJSON(r, &tunnelRaw); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	// Normalize the tunnel
	raw := &configv2.RawConfig{
		Tunnels: []configv2.TunnelRaw{tunnelRaw},
	}
	canonical, err := configv2.Normalize(raw)
	if err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	if len(canonical.Tunnels) == 0 {
		utils.WriteJSON(w, errors.New("no tunnel produced"), nil)
		return
	}

	tunnel := canonical.Tunnels[0]
	if err := s.backend.MountTunnel(tunnel); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	utils.WriteJSON(w, nil, map[string]string{
		"status": "ok",
		"name":   tunnel.Name,
		"id":     tunnel.ID,
	})
}
