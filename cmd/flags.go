package main

import (
	"flag"
	"fmt"
	"os"
)

type ClientFlags struct {
	RunMode        string
	ConfigFileName string
	LogFormat      string
}

func (f *ClientFlags) Parse() {
	flag.StringVar(&f.RunMode, "mode", "agent", "optional: agent/server/validate")
	flag.StringVar(&f.ConfigFileName, "c", "./config.json", "default name of config file")
	flag.StringVar(&f.LogFormat, "log-format", "text", "log output format: text|json (json for host-managed mode)")
	flag.Parse()

	if f.LogFormat != "text" && f.LogFormat != "json" {
		fmt.Fprintf(os.Stderr, "invalid -log-format: %q (must be text or json)\n", f.LogFormat)
		os.Exit(1)
	}
}
