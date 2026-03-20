package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/agent/configv2"
	"github.com/net-agent/remotework/utils"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, nil, map[string]interface{}{
		"running": s.hub.IsRunning(),
	})
}

func (s *Server) handleNetworks(w http.ResponseWriter, r *http.Request) {
	reports, err := s.hub.GetAllNetworkState()
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
	states, err := s.hub.GetAllServiceState()
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
			PID:         port.PID,
			ProcessName: port.ProcessName,
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

	reports, err := s.hub.GetAllNetworkState()
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

	streamStates := s.hub.GetDataStreamState(limit, networks...)

	var dtos []StreamStateDTO
	for _, ds := range streamStates {
		if ds == nil {
			continue
		}
		for _, st := range ds.Actives {
			dtos = append(dtos, toStreamDTO(ds.Network, st))
		}
		for _, st := range ds.Closeds {
			dtos = append(dtos, toStreamDTO(ds.Network, st))
		}
	}
	if dtos == nil {
		dtos = []StreamStateDTO{}
	}
	utils.WriteJSON(w, nil, dtos)
}

func (s *Server) handlePingAll(w http.ResponseWriter, r *http.Request) {
	reports, err := s.hub.GetPingState()
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

	dur, err := s.hub.PingDomain(network, domain)
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
	if r.Header.Get("X-Confirm") != "yes" {
		utils.WriteJSON(w, errors.New("missing header X-Confirm: yes"), nil)
		return
	}
	utils.WriteJSON(w, nil, map[string]string{"status": "stopping"})
	go s.hub.Stop()
}

// handleAddLink dynamically adds a link (network) at runtime.
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

	if err := s.hub.MountLink(req.Alias, link); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}
	utils.WriteJSON(w, nil, map[string]string{"status": "ok", "alias": req.Alias})
}

// handleAddTunnel dynamically adds a tunnel (service) at runtime.
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
	if err := s.hub.MountTunnel(tunnel); err != nil {
		utils.WriteJSON(w, err, nil)
		return
	}

	utils.WriteJSON(w, nil, map[string]string{
		"status": "ok",
		"name":   tunnel.Name,
		"id":     tunnel.ID,
	})
}
