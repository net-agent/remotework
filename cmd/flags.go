package main

import "flag"

type ClientFlags struct {
	RunMode        string
	HomePath       string
	ConfigFileName string
}

func (f *ClientFlags) Parse() {
	flag.StringVar(&f.RunMode, "mode", "agent", "optional: agent/server/validate")
	flag.StringVar(&f.HomePath, "home", "", "home path for files")
	flag.StringVar(&f.ConfigFileName, "c", "./config.json", "default name of config file")
	flag.Parse()
}
