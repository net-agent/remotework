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
}

func ListListeningPorts() ([]ListeningPortInfo, error) {
	connections, err := gopsnet.Connections("tcp")
	if err != nil {
		return nil, err
	}
	return buildListeningPorts(connections), nil
}

func buildListeningPorts(connections []gopsnet.ConnectionStat) []ListeningPortInfo {
	items := make([]ListeningPortInfo, 0, len(connections))
	for _, connection := range connections {
		if !isListeningConnection(connection) {
			continue
		}
		if connection.Laddr.Port < 1 || connection.Laddr.Port > 65535 {
			continue
		}

		items = append(items, ListeningPortInfo{
			Port:        uint16(connection.Laddr.Port),
			Protocol:    protocolName(connection.Type),
			PID:         uint32(connection.Pid),
			ProcessName: getProcessName(connection.Pid),
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

func dedupeListeningPorts(items []ListeningPortInfo) []ListeningPortInfo {
	unique := make(map[uint16]ListeningPortInfo, len(items))
	for _, item := range items {
		if _, exists := unique[item.Port]; exists {
			continue
		}
		unique[item.Port] = item
	}

	ports := make([]uint16, 0, len(unique))
	for port := range unique {
		ports = append(ports, port)
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })

	result := make([]ListeningPortInfo, 0, len(ports))
	for _, port := range ports {
		result = append(result, unique[port])
	}
	return result
}
