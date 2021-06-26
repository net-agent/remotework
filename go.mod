module github.com/net-agent/remotework

go 1.15

require (
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/net-agent/cipherconn v1.0.0
	github.com/net-agent/flex v1.0.1
	github.com/net-agent/mixlisten v1.0.1
	github.com/net-agent/socks v1.0.1
)

replace github.com/net-agent/flex => ../flex
