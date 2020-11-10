// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txutils"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gopkg.in/yaml.v3"
)

type Destination struct {
	Address string `yaml:"address"`
	Amount  int64  `yaml:"amount"`
}

type Config struct {
	ShardID      uint32          `yaml:"shard_id"`
	Net          string          `yaml:"net"`
	NodeRPC      txutils.NodeRPC `yaml:"node_rpc"`
	DataFile     string          `yaml:"data_file"`
	SenderSecret string          `yaml:"sender_secret"`
	Destinations []Destination   `yaml:"destinations"`
}

func (cfg *Config) NetParams() *chaincfg.Params {
	return chaincfg.NetName(cfg.Net).Params()
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
