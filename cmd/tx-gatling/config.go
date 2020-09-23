package main

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txutils"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gopkg.in/yaml.v3"
)

type Destination struct {
	Address string `yaml:"address"`
	Amount  int64  `yaml:"amount"`
}

type Config struct {
	Net          string          `yaml:"net"`
	NodeRPC      txutils.NodeRPC `yaml:"node_rpc"`
	DataFile     string          `yaml:"data_file"`
	SenderSecret string          `yaml:"sender_secret"`
	Destinations []Destination   `yaml:"destinations"`
}

func (cfg *Config) NetParams() *chain.Params {
	return chain.NetName(cfg.Net).Params()
}

func parseConfig(path string) (Config, error) {
	rawFile, err := ioutil.ReadFile(path)
	if err != nil {
		return Config{}, errors.Wrap(err, "Unable to read configuration")
	}

	cfg := Config{}
	if err = yaml.Unmarshal(rawFile, &cfg); err != nil {
		return Config{}, errors.Wrap(err, "Unable to decode configuration")
	}

	return cfg, nil
}
