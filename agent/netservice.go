package agent

type Service interface {
	Name() string
	Init() error
	Start() error
	Close() error
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
