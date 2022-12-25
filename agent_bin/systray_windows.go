package main

import (
	"fmt"
	"time"

	"github.com/getlantern/systray"
	"github.com/net-agent/remotework/agent"
)

func releaseSysTray() {
	systray.Quit()
}

func initSysTray(hub *agent.Hub) {
	syslog.Println("net hub is working, click systray to get more infos.")

	go systray.Run(func() {
		systray.SetIcon(icondata)
		systray.SetTitle("init systray title")
		systray.SetTooltip(fmt.Sprintf("Make remotework easy again!\n%v", time.Now()))

		btnNetworkReport := systray.AddMenuItem("查看网络状态(network)", "report network state")
		btnServiceReport := systray.AddMenuItem("查看服务状态(service)", "report service state")
		btnPingReport := systray.AddMenuItem("查看依赖连通状态(ping)", "report ping state")
		btnDataStreamReport := systray.AddMenuItem("查看活跃连接状态(stream)", "report actived data stream")
		btnExit := systray.AddMenuItem("退出", "退出程序")

		for {
			select {
			case <-btnNetworkReport.ClickedCh:
				syslog.Println(hub.GetAllNetworkStateString())
			case <-btnServiceReport.ClickedCh:
				syslog.Println(hub.GetAllServiceStateString())
			case <-btnPingReport.ClickedCh:
				syslog.Println(hub.GetPingStateString())
			case <-btnDataStreamReport.ClickedCh:
				syslog.Println(hub.GetAllDataStreamStateString())
			case <-btnExit.ClickedCh:
				syslog.Println("close with systray command")
				systray.Quit()
				hub.StopServices()
			}
		}
	}, func() {
		syslog.Println("systray exit")
	})
}

var icondata []byte = []byte{
	0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x20, 0x20, 0x00, 0x00, 0x00, 0x00,
	0x20, 0x00, 0xa8, 0x10, 0x00, 0x00, 0x16, 0x00, 0x00, 0x00, 0x28, 0x00,
	0x00, 0x00, 0x20, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x01, 0x00,
	0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x09, 0xc6, 0xf5, 0x1b, 0x34, 0x69, 0x66, 0x6b, 0x5a, 0x24,
	0x00, 0xc8, 0x8a, 0x5a, 0x30, 0xc0, 0x7a, 0x88, 0x7d, 0x56, 0x40, 0xbf,
	0xdf, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x29, 0xc6, 0xf2, 0x50, 0x22, 0xcf,
	0xfe, 0xe9, 0x42, 0x64, 0x5e, 0xff, 0x5b, 0x25, 0x00, 0xff, 0x8b, 0x5b,
	0x30, 0xff, 0x7f, 0x8a, 0x7b, 0xff, 0x5d, 0xd6, 0xfd, 0xcd, 0x5e, 0xd0,
	0xf5, 0x36, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x43, 0xc9, 0xf4, 0xac, 0x46, 0xd4, 0xff, 0xff, 0x4f, 0x5c,
	0x53, 0xff, 0x59, 0x25, 0x00, 0xff, 0x89, 0x5b, 0x30, 0xff, 0x86, 0x88,
	0x75, 0xff, 0x71, 0xdb, 0xfe, 0xff, 0x6f, 0xd3, 0xf7, 0x9e, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x55, 0xc6, 0xf1, 0x12, 0x5c, 0xd1,
	0xf7, 0xe3, 0x61, 0xd8, 0xfc, 0xff, 0x57, 0x55, 0x47, 0xff, 0x58, 0x26,
	0x00, 0xff, 0x88, 0x5b, 0x30, 0xff, 0x8b, 0x85, 0x6e, 0xff, 0x82, 0xde,
	0xfd, 0xff, 0x7f, 0xd8, 0xf7, 0xd9, 0x7f, 0xcc, 0xe5, 0x0a, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x6f, 0xd4, 0xf4, 0x47, 0x73, 0xd7, 0xfa, 0xff, 0x78, 0xda,
	0xf9, 0xff, 0x5b, 0x4f, 0x3b, 0xff, 0x57, 0x27, 0x00, 0xff, 0x86, 0x5a,
	0x30, 0xff, 0x8f, 0x81, 0x68, 0xff, 0x93, 0xe1, 0xfa, 0xff, 0x8e, 0xdd,
	0xfa, 0xfd, 0x8a, 0xd8, 0xf6, 0x3b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x81, 0xd8,
	0xf7, 0x88, 0x87, 0xdc, 0xfc, 0xff, 0x8b, 0xdd, 0xf8, 0xff, 0x5d, 0x49,
	0x34, 0xff, 0x56, 0x28, 0x00, 0xff, 0x85, 0x5a, 0x30, 0xff, 0x91, 0x7e,
	0x62, 0xff, 0xa2, 0xe4, 0xfa, 0xff, 0x9e, 0xe2, 0xfb, 0xff, 0x98, 0xdd,
	0xf6, 0x79, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x7f, 0x7f, 0x7f, 0x02, 0x90, 0xdc, 0xf7, 0xc4, 0x9a, 0xe3,
	0xfd, 0xff, 0x9e, 0xe0, 0xf5, 0xff, 0x5e, 0x45, 0x2c, 0xff, 0x55, 0x29,
	0x00, 0xff, 0x84, 0x59, 0x2f, 0xff, 0x92, 0x7c, 0x5e, 0xff, 0xb1, 0xe8,
	0xf9, 0xff, 0xac, 0xe8, 0xfd, 0xff, 0xa4, 0xe1, 0xf7, 0xb9, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x96, 0xd9,
	0xf7, 0x22, 0xa0, 0xe1, 0xf8, 0xf3, 0xad, 0xe9, 0xff, 0xff, 0xad, 0xe2,
	0xf2, 0xff, 0x5e, 0x40, 0x23, 0xff, 0x55, 0x29, 0x00, 0xff, 0x83, 0x59,
	0x2f, 0xff, 0x93, 0x79, 0x58, 0xff, 0xbf, 0xeb, 0xf8, 0xff, 0xbb, 0xed,
	0xfe, 0xff, 0xb1, 0xe6, 0xf9, 0xec, 0xa7, 0xe2, 0xf5, 0x1a, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa6, 0xe1, 0xf6, 0x56, 0xb1, 0xe8,
	0xfb, 0xff, 0xbf, 0xef, 0xff, 0xff, 0xbc, 0xe2, 0xec, 0xff, 0x5c, 0x3a,
	0x19, 0xff, 0x55, 0x2a, 0x00, 0xff, 0x82, 0x59, 0x2e, 0xff, 0x93, 0x75,
	0x52, 0xff, 0xcc, 0xed, 0xf5, 0xff, 0xc9, 0xf1, 0xff, 0xff, 0xbe, 0xec,
	0xfb, 0xff, 0xb5, 0xe7, 0xf8, 0x4c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0xb0, 0xe6, 0xfa, 0x92, 0xbf, 0xec, 0xfc, 0xff, 0xd0, 0xf5,
	0xff, 0xff, 0xc9, 0xe1, 0xe2, 0xff, 0x5a, 0x34, 0x10, 0xff, 0x55, 0x2b,
	0x00, 0xff, 0x81, 0x58, 0x2d, 0xff, 0x92, 0x71, 0x4c, 0xff, 0xd8, 0xee,
	0xf2, 0xff, 0xd6, 0xf5, 0xff, 0xff, 0xc8, 0xef, 0xfc, 0xff, 0xbb, 0xea,
	0xf9, 0x87, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x7f, 0x7f, 0x7f, 0x02, 0xb8, 0xe9,
	0xf9, 0xc6, 0xca, 0xef, 0xfd, 0xff, 0xde, 0xfb, 0xff, 0xff, 0xd3, 0xde,
	0xda, 0xff, 0x58, 0x30, 0x0a, 0xff, 0x55, 0x2c, 0x00, 0xff, 0x7f, 0x58,
	0x2c, 0xff, 0x91, 0x6d, 0x47, 0xff, 0xe1, 0xed, 0xeb, 0xff, 0xdf, 0xf9,
	0xff, 0xff, 0xd0, 0xf1, 0xfc, 0xff, 0xc1, 0xea, 0xfb, 0xbd, 0x00, 0x00,
	0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0xb2, 0xe5, 0xf6, 0x1e, 0xbd, 0xeb, 0xfa, 0xf0, 0xd0, 0xf1,
	0xfd, 0xff, 0xe7, 0xfe, 0xff, 0xff, 0xda, 0xd9, 0xd4, 0xff, 0x56, 0x2d,
	0x07, 0xff, 0x55, 0x2c, 0x00, 0xff, 0x7e, 0x57, 0x2a, 0xff, 0x8d, 0x68,
	0x41, 0xff, 0xe6, 0xea, 0xe4, 0xff, 0xe5, 0xfb, 0xff, 0xff, 0xd3, 0xf2,
	0xfd, 0xff, 0xc4, 0xeb, 0xfa, 0xe9, 0xb9, 0xe8, 0xf3, 0x16, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xac, 0xe3,
	0xf8, 0x4a, 0xbe, 0xec, 0xfb, 0xff, 0xd1, 0xf1, 0xfd, 0xff, 0xe9, 0xff,
	0xff, 0xff, 0xd7, 0xd4, 0xce, 0xff, 0x54, 0x2a, 0x04, 0xff, 0x55, 0x2c,
	0x00, 0xff, 0x7e, 0x55, 0x28, 0xff, 0x8a, 0x63, 0x3a, 0xff, 0xe1, 0xe4,
	0xdf, 0xff, 0xe4, 0xfc, 0xff, 0xff, 0xd3, 0xf2, 0xfc, 0xff, 0xc2, 0xec,
	0xfb, 0xfe, 0xb6, 0xe8, 0xf7, 0x42, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa6, 0xe3, 0xf8, 0x79, 0xbb, 0xeb,
	0xfb, 0xff, 0xcc, 0xf0, 0xfd, 0xff, 0xe2, 0xfd, 0xff, 0xff, 0xca, 0xce,
	0xc9, 0xff, 0x52, 0x27, 0x03, 0xff, 0x56, 0x2c, 0x00, 0xff, 0x7c, 0x54,
	0x26, 0xff, 0x87, 0x60, 0x35, 0xff, 0xd4, 0xdf, 0xd8, 0xff, 0xdd, 0xfb,
	0xff, 0xff, 0xce, 0xf0, 0xfc, 0xff, 0xc0, 0xec, 0xfc, 0xff, 0xb0, 0xe8,
	0xf8, 0x6e, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0xa0, 0xe1, 0xf9, 0xa9, 0xb3, 0xe8, 0xfb, 0xff, 0xc2, 0xed,
	0xfc, 0xff, 0xd6, 0xfa, 0xff, 0xff, 0xbb, 0xc7, 0xc3, 0xff, 0x50, 0x25,
	0x01, 0xff, 0x56, 0x2c, 0x00, 0xff, 0x7b, 0x53, 0x23, 0xff, 0x84, 0x5c,
	0x2f, 0xff, 0xc7, 0xd7, 0xd1, 0xff, 0xd3, 0xf7, 0xff, 0xff, 0xc5, 0xee,
	0xfc, 0xff, 0xb9, 0xeb, 0xfb, 0xff, 0xab, 0xe5, 0xf9, 0xa0, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x66, 0xcc, 0xcc, 0x05, 0x96, 0xdd,
	0xf7, 0xcd, 0xa8, 0xe5, 0xfa, 0xff, 0xb5, 0xe9, 0xfb, 0xff, 0xc6, 0xf7,
	0xff, 0xff, 0xab, 0xbf, 0xbd, 0xff, 0x50, 0x24, 0x00, 0xff, 0x56, 0x2c,
	0x00, 0xff, 0x79, 0x51, 0x20, 0xff, 0x81, 0x59, 0x2a, 0xff, 0xb8, 0xce,
	0xca, 0xff, 0xc6, 0xf4, 0xff, 0xff, 0xb9, 0xea, 0xfb, 0xff, 0xaf, 0xe7,
	0xfb, 0xff, 0xa0, 0xe1, 0xf8, 0xc5, 0x7f, 0x7f, 0x7f, 0x02, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x7f, 0xd1, 0xf6, 0x1c, 0x8b, 0xdb, 0xf7, 0xee, 0x9b, 0xe1,
	0xf9, 0xff, 0xa6, 0xe4, 0xfa, 0xff, 0xb4, 0xf2, 0xff, 0xff, 0x9b, 0xb7,
	0xb6, 0xff, 0x50, 0x22, 0x00, 0xff, 0x56, 0x2c, 0x00, 0xff, 0x77, 0x4e,
	0x1b, 0xff, 0x80, 0x55, 0x25, 0xff, 0xab, 0xc6, 0xc2, 0xff, 0xb7, 0xf1,
	0xff, 0xff, 0xad, 0xe6, 0xfb, 0xff, 0xa3, 0xe4, 0xfa, 0xff, 0x98, 0xdf,
	0xf7, 0xe7, 0x8c, 0xd9, 0xf2, 0x14, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x6e, 0xd3,
	0xf6, 0x3a, 0x7d, 0xd7, 0xf8, 0xfd, 0x8b, 0xdc, 0xf9, 0xff, 0x96, 0xde,
	0xf9, 0xff, 0xa1, 0xec, 0xff, 0xff, 0x8c, 0xaf, 0xb1, 0xff, 0x50, 0x22,
	0x00, 0xff, 0x55, 0x2c, 0x00, 0xff, 0x74, 0x4c, 0x18, 0xff, 0x7e, 0x52,
	0x1f, 0xff, 0x9f, 0xbe, 0xbc, 0xff, 0xa8, 0xed, 0xff, 0xff, 0xa0, 0xe2,
	0xf9, 0xff, 0x98, 0xdf, 0xf9, 0xff, 0x8d, 0xda, 0xf8, 0xfb, 0x85, 0xd6,
	0xf5, 0x32, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x59, 0xd5, 0xfc, 0x5c, 0x6b, 0xdd,
	0xff, 0xff, 0x7a, 0xe0, 0xff, 0xff, 0x84, 0xdf, 0xff, 0xff, 0x8e, 0xe8,
	0xff, 0xff, 0x7d, 0xa6, 0xac, 0xff, 0x50, 0x21, 0x00, 0xff, 0x55, 0x2c,
	0x00, 0xff, 0x72, 0x49, 0x14, 0xff, 0x7b, 0x4e, 0x19, 0xff, 0x93, 0xb6,
	0xb5, 0xff, 0x9a, 0xe9, 0xff, 0xff, 0x92, 0xe1, 0xfe, 0xff, 0x8a, 0xe2,
	0xff, 0xff, 0x80, 0xe1, 0xff, 0xff, 0x74, 0xdb, 0xfc, 0x4f, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x71, 0x92, 0x8e, 0x8a, 0x69, 0x97, 0x94, 0xff, 0x67, 0xaa,
	0xb3, 0xff, 0x6e, 0xc2, 0xdb, 0xff, 0x78, 0xe1, 0xf9, 0xfe, 0x6d, 0xa2,
	0xa9, 0xfe, 0x52, 0x20, 0x00, 0xff, 0x55, 0x2c, 0x00, 0xff, 0x6f, 0x46,
	0x11, 0xff, 0x7a, 0x4a, 0x13, 0xff, 0x87, 0xb1, 0xb0, 0xff, 0x89, 0xe5,
	0xfd, 0xff, 0x84, 0xd0, 0xe9, 0xff, 0x80, 0xc0, 0xcd, 0xff, 0x7f, 0xab,
	0xaf, 0xff, 0x7d, 0x96, 0x92, 0x7a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x90, 0x60,
	0x34, 0xaf, 0x77, 0x45, 0x0b, 0xff, 0x64, 0x35, 0x00, 0xff, 0x5b, 0x37,
	0x0c, 0xff, 0x5a, 0x4c, 0x34, 0xff, 0x59, 0x4d, 0x37, 0xff, 0x54, 0x28,
	0x00, 0xff, 0x55, 0x2c, 0x00, 0xff, 0x6c, 0x42, 0x0d, 0xff, 0x78, 0x4d,
	0x17, 0xff, 0x7e, 0x70, 0x4e, 0xff, 0x82, 0x77, 0x58, 0xff, 0x87, 0x67,
	0x42, 0xff, 0x8a, 0x61, 0x38, 0xff, 0x8d, 0x5f, 0x33, 0xff, 0x8d, 0x5e,
	0x34, 0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x8b, 0x67, 0x41, 0xc9, 0x76, 0x4e,
	0x1a, 0xff, 0x65, 0x3b, 0x00, 0xff, 0x5c, 0x30, 0x00, 0xff, 0x56, 0x27,
	0x00, 0xff, 0x54, 0x24, 0x00, 0xff, 0x54, 0x2c, 0x00, 0xff, 0x55, 0x2c,
	0x00, 0xff, 0x69, 0x40, 0x07, 0xff, 0x75, 0x4d, 0x17, 0xff, 0x7c, 0x50,
	0x1c, 0xff, 0x82, 0x55, 0x25, 0xff, 0x87, 0x5d, 0x33, 0xff, 0x89, 0x62,
	0x3a, 0xff, 0x8b, 0x65, 0x3e, 0xff, 0x8a, 0x65, 0x3f, 0xc9, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x8c, 0x65, 0x40, 0x5b, 0x75, 0x4c, 0x19, 0xcb, 0x65, 0x3c,
	0x02, 0xff, 0x5c, 0x33, 0x00, 0xff, 0x57, 0x2e, 0x00, 0xff, 0x55, 0x2b,
	0x00, 0xff, 0x54, 0x2b, 0x00, 0xff, 0x55, 0x2c, 0x00, 0xff, 0x65, 0x3c,
	0x03, 0xff, 0x71, 0x49, 0x12, 0xff, 0x7a, 0x52, 0x20, 0xff, 0x81, 0x5a,
	0x2c, 0xff, 0x85, 0x5f, 0x34, 0xff, 0x89, 0x62, 0x3a, 0xff, 0x8a, 0x63,
	0x3d, 0xd2, 0x89, 0x64, 0x3d, 0x68, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x6d, 0x49, 0x00, 0x07, 0x61, 0x39, 0x00, 0x51, 0x57, 0x2e,
	0x00, 0xb5, 0x56, 0x2d, 0x01, 0xfd, 0x54, 0x2a, 0x00, 0xff, 0x54, 0x2b,
	0x00, 0xff, 0x54, 0x2b, 0x00, 0xff, 0x61, 0x38, 0x00, 0xff, 0x6f, 0x46,
	0x0d, 0xff, 0x78, 0x50, 0x1d, 0xff, 0x7d, 0x56, 0x27, 0xfc, 0x84, 0x5d,
	0x33, 0xb4, 0x86, 0x60, 0x38, 0x52, 0x7f, 0x4c, 0x33, 0x0a, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x60, 0x40, 0x20, 0x08, 0x77, 0x4f,
	0x22, 0xde, 0x59, 0x2f, 0x00, 0xff, 0x54, 0x2b, 0x00, 0xff, 0x53, 0x2a,
	0x00, 0xff, 0x5c, 0x33, 0x00, 0xff, 0x6a, 0x42, 0x08, 0xff, 0x6f, 0x47,
	0x0f, 0xff, 0x6a, 0x44, 0x09, 0xdb, 0x55, 0x2a, 0x00, 0x06, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x55, 0x55, 0x00, 0x03, 0x7f, 0x58, 0x28, 0xc4, 0x5f, 0x36,
	0x00, 0xff, 0x55, 0x2b, 0x00, 0xff, 0x54, 0x2b, 0x01, 0xff, 0x61, 0x39,
	0x03, 0xff, 0x69, 0x41, 0x05, 0xff, 0x68, 0x40, 0x04, 0xff, 0x66, 0x3e,
	0x04, 0xcf, 0x33, 0x33, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x76, 0x52, 0x24, 0x1c, 0x5a, 0x31, 0x00, 0xb0, 0x54, 0x2b,
	0x00, 0xff, 0x55, 0x2d, 0x01, 0xff, 0x68, 0x40, 0x05, 0xff, 0x68, 0x40,
	0x04, 0xff, 0x65, 0x3c, 0x03, 0xb3, 0x65, 0x3b, 0x00, 0x2b, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x52, 0x29, 0x00, 0x88, 0x53, 0x29, 0x00, 0x75, 0x55, 0x2c,
	0x00, 0xa8, 0x68, 0x3f, 0x05, 0xa9, 0x5f, 0x36, 0x02, 0x8e, 0x51, 0x29,
	0x00, 0x71, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x53, 0x2a,
	0x00, 0x68, 0x52, 0x2a, 0x00, 0x92, 0x4c, 0x19, 0x00, 0x0a, 0x60, 0x30,
	0x00, 0x10, 0x53, 0x2a, 0x00, 0xa2, 0x51, 0x28, 0x00, 0x52, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x54, 0x29,
	0x00, 0x7d, 0x52, 0x2a, 0x00, 0xb0, 0x53, 0x2a, 0x00, 0xb1, 0x53, 0x29,
	0x00, 0x71, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe,
	0x7f, 0xff, 0xff, 0xf8, 0x1f, 0xff, 0xff, 0xf0, 0x0f, 0xff, 0xff, 0xf0,
	0x0f, 0xff, 0xff, 0xf0, 0x0f, 0xff, 0xff, 0xe0, 0x0f, 0xff, 0xff, 0xe0,
	0x07, 0xff, 0xff, 0xe0, 0x07, 0xff, 0xff, 0xe0, 0x07, 0xff, 0xff, 0xc0,
	0x03, 0xff, 0xff, 0xc0, 0x03, 0xff, 0xff, 0xc0, 0x03, 0xff, 0xff, 0xc0,
	0x03, 0xff, 0xff, 0xc0, 0x03, 0xff, 0xff, 0x80, 0x01, 0xff, 0xff, 0x80,
	0x01, 0xff, 0xff, 0x80, 0x01, 0xff, 0xff, 0x80, 0x01, 0xff, 0xff, 0x80,
	0x01, 0xff, 0xff, 0x00, 0x01, 0xff, 0xff, 0x00, 0x00, 0xff, 0xff, 0x00,
	0x00, 0xff, 0xff, 0x80, 0x01, 0xff, 0xff, 0xe0, 0x07, 0xff, 0xff, 0xf0,
	0x0f, 0xff, 0xff, 0xf0, 0x0f, 0xff, 0xff, 0xf8, 0x1f, 0xff, 0xff, 0xfa,
	0x3f, 0xff, 0xff, 0xfd, 0xbf, 0xff, 0xff, 0xfe, 0x7f, 0xff, 0xff, 0xff,
	0xff, 0xff,
}
