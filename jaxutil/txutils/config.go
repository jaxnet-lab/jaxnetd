// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txutils

import (
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

type NodeRPC struct {
	Host string `json:"host" yaml:"host" toml:"host"`
	User string `json:"user" yaml:"user" toml:"user"`
	Pass string `json:"pass" yaml:"pass" toml:"pass"`
}

type ManagerCfg struct {
	Net        string  `json:"net" yaml:"net" toml:"net"`
	ShardID    uint32  `json:"shard_id" yaml:"shard_id" toml:"shard_id"`
	RPC        NodeRPC `json:"rpc" yaml:"rpc" toml:"rpc"`
	PrivateKey string  `json:"private_key" yaml:"private_key" toml:"private_key"`
}

func (cfg *ManagerCfg) NetParams() *chaincfg.Params {
	return chaincfg.NetName(cfg.Net).Params()
}
