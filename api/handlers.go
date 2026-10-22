package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/net-agent/remotework/configv2"
	"github.com/net-agent/remotework/utils"
	"github.com/net-agent/remotework/vnet"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, nil, statusDTO{
		Running:     s.backend.IsRunning(),
		Mode:        s.meta.Mode,
		Version:     s.meta.Version,
		Commit:      s.meta.Commit,
		Protocol:    s.meta.Protocol,
		StartedAtMs: s.startedAt.UnixMilli(),
		ConfigPath:  s.meta.ConfigPath,
		// 配置错误可能回显带 authcode/userinfo 的畸形 URL，对外必须脱敏
		ConfigError: redactText(s.meta.ConfigError),
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
	// 短 TTL 缓存：枚举监听端口是系统调用（darwin 上会 exec netstat），
	// 不能被认证客户端高频放大；2s 内的重复请求复用同一份结果。
	now := time.Now()
	s.portsMu.Lock()
	if s.portsOK && now.Sub(s.portsAt) < listeningPortsCacheTTL {
		cached := s.portsData
		s.portsMu.Unlock()
		utils.WriteJSON(w, nil, cached)
		return
	}
	s.portsMu.Unlock()

	listPorts := s.listPorts
	if listPorts == nil {
		listPorts = utils.ListListeningPorts
	}
	ports, err := listPorts()
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

	s.portsMu.Lock()
	s.portsData = dtos
	s.portsAt = now
	s.portsOK = true
	s.portsMu.Unlock()

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
		// 一次批量标注本网络全部流（含已关闭流的历史透传），
		// 后端实现 StreamAnnotator 时只枚举一次服务列表
		indexes := make([]int64, 0, len(ds.Actives)+len(ds.Closeds))
		for _, st := range ds.Actives {
			if st != nil {
				indexes = append(indexes, int64(st.Index))
			}
		}
		for _, st := range ds.Closeds {
			if st != nil {
				indexes = append(indexes, int64(st.Index))
			}
		}
		annotations := annotateStreams(s.backend, ds.Network, indexes)

		// 活跃流采样实时速率，并标注归属服务
		rates := s.meter.sample(ds.Network, ds.Actives, now)
		for _, st := range ds.Actives {
			if st == nil {
				continue
			}
			dto := toStreamDTO(ds.Network, st)
			if rate, ok := rates[st.Index]; ok {
				dto.ReadBps = rate[0]
				dto.WriteBps = rate[1]
			}
			if ann, ok := annotations[int64(st.Index)]; ok {
				dto.ServiceName = ann.ServiceName
				dto.PipeID = ann.PipeID
			}
			s.resolver.enrich(ds.Network, st, &dto)
			dtos = append(dtos, dto)
		}
		for _, st := range ds.Closeds {
			if st == nil {
				continue
			}
			dto := toStreamDTO(ds.Network, st)
			if ann, ok := annotations[int64(st.Index)]; ok {
				dto.ServiceName = ann.ServiceName
				dto.PipeID = ann.PipeID
			}
			s.resolver.enrich(ds.Network, st, &dto)
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

// pingAllResult 批量 ping 的后台结果
type pingAllResult struct {
	reports []*vnet.PingReport
	err     error
}

// handlePingAll 汇总 ping。后端探测是串行的（每 domain 最多 3s），
// 因此限制并发执行数并给整请求设置等待上限：
// 槽位由后台 goroutine 在真正结束时归还，超时返回不会造成无界 goroutine。
func (s *Server) handlePingAll(w http.ResponseWriter, r *http.Request) {
	if !s.acquirePingSlot() {
		utils.WriteJSON(w, errors.New("too many concurrent ping requests"), nil)
		return
	}
	ch := make(chan pingAllResult, 1)
	go func() {
		defer s.releasePingSlot()
		reports, err := s.backend.GetPingState()
		ch <- pingAllResult{reports: reports, err: err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			utils.WriteJSON(w, res.err, nil)
			return
		}
		dtos := make([]PingResultDTO, 0, len(res.reports))
		for _, report := range res.reports {
			dtos = append(dtos, toPingDTO(report))
		}
		utils.WriteJSON(w, nil, dtos)
	case <-time.After(pingRequestTimeout):
		utils.WriteJSON(w, errors.New("ping request timeout"), nil)
	case <-r.Context().Done():
		// 客户端断开：不写响应，后台探测结束后自行归还槽位
	}
}

// pingSingleResult 单 domain ping 的后台结果
type pingSingleResult struct {
	dur time.Duration
	err error
}

func (s *Server) handlePingSingle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	network := vars["network"]
	domain := vars["domain"]

	if !s.acquirePingSlot() {
		utils.WriteJSON(w, errors.New("too many concurrent ping requests"), nil)
		return
	}
	ch := make(chan pingSingleResult, 1)
	go func() {
		defer s.releasePingSlot()
		dur, err := s.backend.PingDomain(network, domain)
		ch <- pingSingleResult{dur: dur, err: err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			utils.WriteJSON(w, res.err, nil)
			return
		}
		utils.WriteJSON(w, nil, map[string]interface{}{
			"network": network,
			"domain":  domain,
			"result":  res.dur.String(),
		})
	case <-time.After(pingSingleTimeout):
		utils.WriteJSON(w, errors.New("ping request timeout"), nil)
	case <-r.Context().Done():
	}
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

	// 动态挂载路径不能只依赖 Normalize：空 id/name/listen/target 会产出
	// 名为 "tunnel/" 的插件并永久 Pending，HTTP 却返回成功（假成功）。
	for _, field := range []struct {
		name  string
		value string
	}{
		{"id", tunnelRaw.ID},
		{"name", tunnelRaw.Name},
		{"listen", tunnelRaw.Listen},
		{"target", tunnelRaw.Target},
	} {
		if strings.TrimSpace(field.value) == "" {
			utils.WriteJSON(w, fmt.Errorf("tunnel %s is required", field.name), nil)
			return
		}
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
