package agent

import "fmt"

func color(color int, info string) string { return fmt.Sprintf("\x1b[%dm%v\x1b[0m", color, info) }
func Green(info string) string            { return color(32, info) }
func Red(info string) string              { return color(31, info) }
func Yellow(info string) string           { return color(33, info) }
