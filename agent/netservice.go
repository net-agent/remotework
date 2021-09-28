package agent

type Service interface {
	Name() string
	Network() string
	Init() error
	Start() error
	Close() error
	Update() error // 依赖的netnode重连后，能够更新runner
	Report() ReportInfo
}

type ReportInfos []ReportInfo

type ReportInfo struct {
	Name    string
	State   string
	Listen  string
	Target  string
	Actives int32
	Dones   int32
}
