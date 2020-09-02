package txmanager

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
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
	switch cfg.Net {
	case "simnet":
		return &chaincfg.SimNetParams
	case "testnet":
		return &chaincfg.TestNet3Params
	case "mainnet":
		return &chaincfg.MainNetParams
	}

	return &chaincfg.Params{}
}
