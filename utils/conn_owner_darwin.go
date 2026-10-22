//go:build darwin

package utils

import (
	"net/netip"
	"os/exec"
	"strconv"
	"strings"

	gopsnet "github.com/shirou/gopsutil/v4/net"
)

// netstatTCP 做成包内变量便于单测替换，避免测试路径触发真实系统调用
var netstatTCP = func() ([]byte, error) {
	return exec.Command("netstat", "-anv", "-p", "tcp").Output()
}

// listTCPConnections 在 darwin 上解析 netstat（无 lsof 的本用户权限盲区，
// TIME_WAIT 残留仍带属主）；其余平台见 conn_owner_notdarwin.go。
// 包内变量便于单测替换。
var listTCPConnections = func() ([]gopsnet.ConnectionStat, error) {
	out, err := netstatTCP()
	if err != nil {
		return nil, err
	}
	return parseNetstatTCP(out), nil
}

// darwin 端点表选 netstat 而非 gopsutil 的原因：gopsutil 在 macOS 上的
// Connections("tcp") 是 shell 到 lsof 实现的，非 root 的 lsof 只能看到本用户进程，
// 特权/其他用户进程的 socket 行在表中根本不存在（权限盲区）。
// `netstat -anv -p tcp` 无此限制（所有 socket 均携带 name:pid），且已关闭连接的
// TIME_WAIT 残留在窗口期内仍保留属主，毫秒级短连接关闭后由此仍可反查。

// parseNetstatTCP 解析 macOS `netstat -anv -p tcp` 输出为连接行（复用
// buildTCPLocalEndpoints 的去重/偏好逻辑）。输出列形如：
//
//	Proto Recv-Q Send-Q Local Address Foreign Address (state) rxbytes txbytes rhiwat shiwat process:pid state options gencnt flags flags1 usecnt rtncnt fltrs
//
// 进程名被截断展示且可能含空格（如 "Google Chrome He:24954"），故 proc 列按
// fields[10:len-8] 区间拼接（尾部 8 列均为无空格数值/十六进制）。
// 输出经 -p tcp 过滤，所有行均为 TCP，置 Type=1（与 gopsutil SOCK_STREAM 约定一致，
// 使 protocolName 等下游得到正确协议名）
func parseNetstatTCP(out []byte) []gopsnet.ConnectionStat {
	var rows []gopsnet.ConnectionStat
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 19 || !strings.HasPrefix(fields[0], "tcp") {
			continue
		}
		ip, port, ok := parseNetstatTCPAddr(fields[3], fields[0])
		if !ok {
			continue
		}
		var pid int32
		procCol := strings.Join(fields[10:len(fields)-8], " ")
		if i := strings.LastIndexByte(procCol, ':'); i >= 0 {
			if v, err := strconv.ParseInt(procCol[i+1:], 10, 32); err == nil {
				pid = int32(v)
			}
		}
		rows = append(rows, gopsnet.ConnectionStat{
			Type:   1,
			Status: fields[5],
			Pid:    pid,
			Laddr:  gopsnet.Addr{IP: ip, Port: port},
		})
	}
	return rows
}

// parseNetstatTCPAddr 解析 netstat 的 address.port 文本（IPv4 与 IPv6 均为点分端口，
// 如 127.0.0.1.21080、::1.8080、::1%lo0.8080）。通配监听地址（*.8080）按 proto
// 映射为 0.0.0.0（tcp4）或 ::（tcp6/tcp46 双栈），与监听 DTO 文档示例一致；
// 无法解析的非法行 ok=false
func parseNetstatTCPAddr(text, proto string) (string, uint32, bool) {
	i := strings.LastIndexByte(text, '.')
	if i < 0 {
		return "", 0, false
	}
	host, portText := text[:i], text[i+1:]
	port, err := strconv.ParseUint(portText, 10, 32)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}
	if host == "*" {
		if proto == "tcp4" {
			return "0.0.0.0", uint32(port), true
		}
		return "::", uint32(port), true
	}
	if j := strings.IndexByte(host, '%'); j >= 0 {
		host = host[:j] // 剥去 IPv6 zone
	}
	if _, err := netip.ParseAddr(host); err != nil {
		return "", 0, false
	}
	return host, uint32(port), true
}
