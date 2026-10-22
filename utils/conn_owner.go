package utils

import (
	"net/netip"
	"strings"

	gopsnet "github.com/shirou/gopsutil/v4/net"
)

// TCPEndpointKey 本机 TCP 端点（连接表中一行的本机侧地址），用于按地址反查进程归属
type TCPEndpointKey struct {
	IP   string
	Port uint16
}

// listTCPConnections 由平台文件提供（darwin: netstat；其余平台: gopsutil），
// 定义见 conn_owner_darwin.go / conn_owner_notdarwin.go。包内变量便于单测替换。

// GetProcessName 查询进程名；pid 非法或查询失败返回 ""
func GetProcessName(pid int32) string { return getProcessName(pid) }

// GetProcessStartMs 查询进程启动时刻（Unix 毫秒）；pid 非法或获取失败返回 0
func GetProcessStartMs(pid int32) int64 { return getProcessStartMs(pid) }

// ListTCPLocalEndpoints 枚举本机 TCP 连接表，返回 本机端点→PID 映射。
// 只登记各连接的本机侧地址（Laddr），因此客户端连接的临时端口与服务端监听端口
// 都以其真实属主登记；同一端点出现多行（TIME_WAIT 残留与端口复用并存）时，
// 优先保留 ESTABLISHED 且 PID 合法的一行。
func ListTCPLocalEndpoints() (map[TCPEndpointKey]int32, error) {
	connections, err := listTCPConnections()
	if err != nil {
		return nil, err
	}
	return buildTCPLocalEndpoints(connections), nil
}

// endpointRow 端点归属候选行
type endpointRow struct {
	pid         int32
	established bool
}

func buildTCPLocalEndpoints(connections []gopsnet.ConnectionStat) map[TCPEndpointKey]int32 {
	owners := make(map[TCPEndpointKey]endpointRow, len(connections))
	for _, connection := range connections {
		if connection.Laddr.Port < 1 || connection.Laddr.Port > 65535 {
			continue
		}
		key := TCPEndpointKey{
			IP:   normalizeIPText(connection.Laddr.IP),
			Port: uint16(connection.Laddr.Port),
		}
		candidate := endpointRow{pid: connection.Pid, established: isEstablishedConnection(connection)}
		if current, ok := owners[key]; !ok || endpointRowPreferred(candidate, current) {
			owners[key] = candidate
		}
	}

	result := make(map[TCPEndpointKey]int32, len(owners))
	for key, row := range owners {
		result[key] = row.pid
	}
	return result
}

// endpointRowPreferred 判定 candidate 是否比 current 更能代表端点属主：
// ESTABLISHED 行优先；状态相同时 PID 合法者优先
func endpointRowPreferred(candidate, current endpointRow) bool {
	if candidate.established != current.established {
		return candidate.established
	}
	return current.pid <= 0 && candidate.pid > 0
}

func isEstablishedConnection(connection gopsnet.ConnectionStat) bool {
	return strings.EqualFold(strings.TrimSpace(connection.Status), "ESTABLISHED")
}

// normalizeIPText 归一化 IP 文本（剥去 IPv4-in-IPv6 映射包装），无法解析时原样返回
func normalizeIPText(text string) string {
	ip, err := netip.ParseAddr(text)
	if err != nil {
		return text
	}
	return ip.Unmap().String()
}
