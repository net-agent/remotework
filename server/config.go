package main

import (
	"github.com/net-agent/remotework/utils"
)

type Config struct {
	Server struct {
		Listen          string `json:"listen"`
		Password        string `json:"password"`
		WebsocketPath   string `json:"wsPath"`
		WebsocketEnable bool   `json:"wsEnable"`
	} `json:"server"`
}

func NewConfig(jsonfile string) (*Config, error) {
	cfg := &Config{}

	err := utils.LoadJSONFile(jsonfile, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
