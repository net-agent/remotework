module github.com/net-agent/remotework

go 1.15

require (
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/net-agent/flex v0.0.0-00010101000000-000000000000
	github.com/net-agent/mixlisten v1.0.2
	github.com/net-agent/socks v1.0.1
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22
)

replace github.com/net-agent/flex => ../flex
