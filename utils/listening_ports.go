package utils

import (
	"sort"
	"strings"

	gopsnet "github.com/shirou/gopsutil/v4/net"
	gopsprocess "github.com/shirou/gopsutil/v4/process"
)

type ListeningPortInfo struct {
	Port        uint16
	Protocol    string
	PID         uint32
	ProcessName string
	// Address 为监听地址，取自连接本地地址（如 "0.0.0.0"、"::"、"127.0.0.1"）
	Address string
	// ProcStartMs 为进程启动时间（Unix 毫秒），pid 非法或获取失败时为 0
	ProcStartMs int64
}

// getProcessStartMs 获取进程启动时间（Unix 毫秒）。做成包内变量便于单测替换，
// 避免测试路径触发真实系统调用。pid<=0 或获取失败时返回 0。
var getProcessStartMs = func(pid int32) int64 {
	if pid <= 0 {
		return 0
	}
	process, err := gopsprocess.NewProcess(pid)
	if err != nil {
		return 0
	}
	createTime, err := process.CreateTime()
	if err != nil {
		return 0
	}
	return createTime
}

// ListListeningPorts 枚举本机 TCP 监听端口。连接表数据源见 listTCPConnections：
// darwin 解析 netstat（无 lsof 的本用户权限盲区，root/其他用户进程的监听可见），
// 其余平台维持 gopsutil
func ListListeningPorts() ([]ListeningPortInfo, error) {
	connections, err := listTCPConnections()
	if err != nil {
		return nil, err
	}
	return buildListeningPorts(connections), nil
}

func buildListeningPorts(connections []gopsnet.ConnectionStat) []ListeningPortInfo {
	items := make([]ListeningPortInfo, 0, len(connections))
	// CreateTime 调用成本高，同一次采集内按 pid 缓存，避免每个 socket 重复查询
	startMsCache := make(map[int32]int64)
	for _, connection := range connections {
		if !isListeningConnection(connection) {
			continue
		}
		if connection.Laddr.Port < 1 || connection.Laddr.Port > 65535 {
			continue
		}

		startMs, cached := startMsCache[connection.Pid]
		if !cached {
			startMs = getProcessStartMs(connection.Pid)
			startMsCache[connection.Pid] = startMs
		}

		items = append(items, ListeningPortInfo{
			Port:        uint16(connection.Laddr.Port),
			Protocol:    protocolName(connection.Type),
			PID:         uint32(connection.Pid),
			ProcessName: getProcessName(connection.Pid),
			Address:     connection.Laddr.IP,
			ProcStartMs: startMs,
		})
	}
	return dedupeListeningPorts(items)
}

func isListeningConnection(connection gopsnet.ConnectionStat) bool {
	status := strings.ToUpper(strings.TrimSpace(connection.Status))
	return status == "LISTEN" || status == "LISTENING"
}

func getProcessName(pid int32) string {
	if pid <= 0 {
		return ""
	}
	process, err := gopsprocess.NewProcess(pid)
	if err != nil {
		return ""
	}
	name, err := process.Name()
	if err != nil {
		return ""
	}
	return name
}

func protocolName(kind uint32) string {
	switch kind {
	case 1:
		return "tcp"
	case 2:
		return "udp"
	default:
		return "unknown"
	}
}

// listeningPortKey 为去重键：同端口的不同监听地址（IPv4/IPv6 双栈）或
// 不同进程（SO_REUSEPORT）视为不同监听，只有三者完全一致才合并。
type listeningPortKey struct {
	address string
	port    uint16
	pid     uint32
}

func dedupeListeningPorts(items []ListeningPortInfo) []ListeningPortInfo {
	unique := make(map[listeningPortKey]ListeningPortInfo, len(items))
	for _, item := range items {
		key := listeningPortKey{address: item.Address, port: item.Port, pid: item.PID}
		if _, exists := unique[key]; exists {
			continue
		}
		unique[key] = item
	}

	result := make([]ListeningPortInfo, 0, len(unique))
	for _, item := range unique {
		result = append(result, item)
	}
	// 排序保持确定性：端口升序为主键，同端口按监听地址字典序、再按 PID 升序
	sort.Slice(result, func(i, j int) bool {
		if result[i].Port != result[j].Port {
			return result[i].Port < result[j].Port
		}
		if result[i].Address != result[j].Address {
			return result[i].Address < result[j].Address
		}
		return result[i].PID < result[j].PID
	})
	return result
}
