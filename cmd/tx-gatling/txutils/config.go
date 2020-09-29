package txutils

import (
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincfg"
)

type NodeRPC struct {
	Host string `json:"host" yaml:"host"`
	User string `json:"user" yaml:"user"`
	Pass string `json:"pass" yaml:"pass"`
}

type ManagerCfg struct {
	Net        string  `json:"net" yaml:"net"`
	RPC        NodeRPC `json:"rpc" yaml:"rpc"`
	PrivateKey string  `json:"private_key" yaml:"private_key"`
}

func (cfg *ManagerCfg) NetParams() *chaincfg.Params {
	return chaincfg.NetName(cfg.Net).Params()
}
