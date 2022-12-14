// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/txutils"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gopkg.in/yaml.v3"
)

type Destination struct {
	Address string `yaml:"address"`
	Amount  int64  `yaml:"amount"`
	ShardID uint32 `yaml:"shard_id"`
}

type SendTxsCmd struct {
	SenderSecret string        `yaml:"sender_secret"`
	Destinations []Destination `yaml:"destinations"`
}

type CmdSet struct {
	SendTxs *SendTxsCmd `yaml:"send_txs"`
}

type Config struct {
	ShardID      uint32          `yaml:"shard_id"`
	Net          string          `yaml:"net"`
	NodeRPC      txutils.NodeRPC `yaml:"node_rpc"`
	DataFile     string          `yaml:"data_file"`
	SenderSecret string          `yaml:"sender_secret"`
	Cmd          CmdSet          `yaml:"cmd"`
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
