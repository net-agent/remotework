package main

import "flag"

type ClientFlags struct {
	RunMode        string
	HomePath       string
	ConfigFileName string
	PingDomain     string
	PingClientName string
}

func (f *ClientFlags) Parse() {
	flag.StringVar(&f.RunMode, "mode", "agent", "optional: agent/server/cli")
	flag.StringVar(&f.HomePath, "home", "", "home path for files")
	flag.StringVar(&f.ConfigFileName, "c", "./config.json", "default name of config file")
	flag.StringVar(&f.PingDomain, "ping", "", "<protocol>://<target_domain>:<password>@<host>:<port><path>")
	flag.StringVar(&f.PingClientName, "pingname", "", "upgrade as <pingclient>")
	flag.Parse()
}
