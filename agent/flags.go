package agent

import "flag"

type AgentFlags struct {
	HomePath       string
	ConfigFileName string
	IPCPath        string
}

func (f *AgentFlags) Parse() {
	flag.StringVar(&f.HomePath,
		"home", "", "home path for files")
	flag.StringVar(&f.ConfigFileName,
		"c", "./config.json", "default name of config file")
	flag.StringVar(&f.IPCPath,
		"ipc", "", "ipc path for launcher")
	flag.Parse()
}
