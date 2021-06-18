package main

import (
	"github.com/net-agent/flex"
)

func main() {
	sw := flex.NewSwitcher(nil)
	sw.Run("localhost:2038")
}
