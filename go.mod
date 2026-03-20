module github.com/net-agent/remotework

go 1.24.0

require (
	github.com/BurntSushi/toml v0.4.1
	github.com/dustin/go-humanize v1.0.1
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.4.2
	github.com/net-agent/cipherconn v1.0.2
	github.com/net-agent/flex/v3 v3.0.1
	github.com/net-agent/socks v1.0.7
	github.com/olekukonko/tablewriter v0.0.5
	github.com/shirou/gopsutil/v4 v4.25.7
	golang.org/x/sys v0.41.0
)

require (
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
)

replace github.com/net-agent/flex/v3 => ../flex
