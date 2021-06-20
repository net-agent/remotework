module github.com/net-agent/remotework

go 1.15

require (
	github.com/net-agent/cipherconn v1.0.0
	github.com/net-agent/flex v1.0.0
	github.com/net-agent/socks v1.0.0
)

replace github.com/net-agent/flex => ../flex

replace github.com/net-agent/socks => ../socks
