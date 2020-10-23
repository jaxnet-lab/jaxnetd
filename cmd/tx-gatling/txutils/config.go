// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txutils

import (
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
)

type NodeRPC struct {
	Host string `json:"host" yaml:"host"`
	User string `json:"user" yaml:"user"`
	Pass string `json:"pass" yaml:"pass"`
}

type ManagerCfg struct {
	Net        string  `json:"net" yaml:"net"`
	ShardID    uint32  `json:"shard_id" yaml:"shard_id"`
	RPC        NodeRPC `json:"rpc" yaml:"rpc"`
	PrivateKey string  `json:"private_key" yaml:"private_key"`
}

func (cfg *ManagerCfg) NetParams() *chaincfg.Params {
	return chaincfg.NetName(cfg.Net).Params()
}
