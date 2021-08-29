package agent

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

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

func PrintReportInfos(out *os.File, reports []ReportInfo) {
	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{"index", "name", "state", "listen", "target", "actives", "dones"})
	for index, info := range reports {
		table.Append([]string{
			fmt.Sprintf("%v", index),
			info.Name,
			info.State,
			info.Listen,
			info.Target,
			fmt.Sprintf("%v", info.Actives),
			fmt.Sprintf("%v", info.Dones),
		})
	}
	table.Render()
}
