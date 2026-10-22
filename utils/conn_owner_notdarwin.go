//go:build !darwin

package utils

import (
	gopsnet "github.com/shirou/gopsutil/v4/net"
)

// listTCPConnections 在非 darwin 平台使用 gopsutil 枚举 TCP 连接表；
// darwin 的 netstat 实现见 conn_owner_darwin.go。包内变量便于单测替换。
var listTCPConnections = func() ([]gopsnet.ConnectionStat, error) {
	return gopsnet.Connections("tcp")
}
