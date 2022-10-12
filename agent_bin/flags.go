package main

import "flag"

type ClientFlags struct {
	HomePath       string
	ConfigFileName string
	IPCPath        string
}

func (f *ClientFlags) Parse() {
	flag.StringVar(&f.HomePath,
		"home", "", "home path for files")
	flag.StringVar(&f.ConfigFileName,
		"c", "./config.json", "default name of config file")
	flag.StringVar(&f.IPCPath,
		"ipc", "", "ipc path for launcher")
	flag.Parse()
}
