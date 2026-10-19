package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// 构建期经 -ldflags -X 注入的版本信息
var (
	version = "dev"
	commit  = ""
)

// protocolVersion 为 sidecar 协议版本（docs/tauri-sidecar-protocol.md）。
// v2 新增：WS 主题订阅（snapshot-on-subscribe / resync / 事件信封 seq+topic）、
// 流生命周期即时事件（stream.opened/closed）、每 tick 计数器快照（network.stats/service.stats）。
const protocolVersion = 2

// managedMode 仅当 -log-format=json（Host 托管模式）时为 true，
// 此时 stdout 承载 C3 控制行与结构化日志；文本模式完全不输出控制行。
var managedMode bool

// readyMsg C3 ready 控制消息，单行 JSON 写 stdout
type readyMsg struct {
	Type        string `json:"type"`
	Protocol    int    `json:"protocol"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	API         string `json:"api"`
	Mode        string `json:"mode"`
	ConfigError string `json:"configError,omitempty"`
	PID         int    `json:"pid"`
}

// fatalMsg C3 fatal 控制消息
type fatalMsg struct {
	Type  string `json:"type"`
	Stage string `json:"stage"`
	Error string `json:"error"`
}

func printControlLine(v interface{}) {
	buf, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Println(string(buf))
}

// printReady 在 API 端口监听就绪后输出唯一一行 ready 消息；
// apiAddr 必须是实际绑定地址（配置 :0 时为系统分配端口）。
func printReady(apiAddr, mode, configError string) {
	if !managedMode {
		return
	}
	printControlLine(readyMsg{
		Type:        "ready",
		Protocol:    protocolVersion,
		Version:     version,
		Commit:      commit,
		API:         apiAddr,
		Mode:        mode,
		ConfigError: configError,
		PID:         os.Getpid(),
	})
}

// fatalExit 输出 fatal 控制消息（仅托管模式）后以 code 退出；
// 同时始终记录 error 日志。
func fatalExit(stage string, code int, err error) {
	if managedMode {
		printControlLine(fatalMsg{Type: "fatal", Stage: stage, Error: err.Error()})
	}
	syslog.Error("fatal", "stage", stage, "err", err)
	os.Exit(code)
}
