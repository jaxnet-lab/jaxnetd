// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package node

import (
	"os"

	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/network/rpc"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
)

type Config struct {
	ConfigFile  string `yaml:"-" short:"C" long:"configfile" description:"Path to configuration file"`
	ShowVersion bool   `yaml:"-" short:"V" long:"version" description:"Display version information and exit"`

	Node             NodeConfig `yaml:"node"`
	DataDir          string     `yaml:"data_dir" short:"b" long:"datadir" description:"Directory to store data"`
	LogDir           string     `yaml:"log_dir" long:"logdir" description:"Directory to log output."`
	CPUProfile       string     `yaml:"cpu_profile" long:"cpuprofile" description:"Write CPU profile to the specified file"`
	DebugLevel       string     `yaml:"debug_level" short:"d" long:"debuglevel" description:"Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available subsystems"`
	DisableStdOutLog bool       `yaml:"disable_std_out_log"`
	Profile          string     `yaml:"profile" long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	DropAddrIndex    bool       `yaml:"drop_addr_index" long:"dropaddrindex" description:"Deletes the address-based transaction index from the database on start up and then exits."`
	DropCfIndex      bool       `yaml:"drop_cf_index" long:"dropcfindex" description:"Deletes the index used for committed filtering (CF) support from the database on start up and then exits."`
	DropTxIndex      bool       `yaml:"drop_tx_index" long:"droptxindex" description:"Deletes the hash-based transaction index from the database on start up and then exits."`
	TorIsolation     bool       `yaml:"tor_isolation" long:"torisolation" description:"Enable Tor stream isolation by randomizing user credentials for each connection."`
	Whitelists       []string   `yaml:"whitelists" long:"whitelist" description:"Add an IP network or IP that will not be banned. (eg. 192.168.1.0/24 or ::1)"`

	// NoPeerBloomFilters bool     `yaml:"no_peer_bloom_filters" long:"nopeerbloomfilters" description:"Disable bloom filtering support"`
	// UserAgentComments  []string `yaml:"user_agent_comments" long:"uacomment" description:"Comment to add to the user agent -- See BIP 14 for more information."`
	// addCheckpoints     []chaincfg.Checkpoint
	// whitelists         []*net.IPNet
}

type NodeConfig struct {
	BeaconChain     cprovider.ChainRuntimeConfig `yaml:"beacon_chain"`
	RPC             rpc.Config                   `yaml:"rpc"`
	P2P             p2p.Config                   `yaml:"p2p"`
	Shards          ShardConfig                  `yaml:"shards"`
	DbType          string                       `yaml:"db_type" long:"dbtype" description:"Database backend to use for the Block Chain"`
	Net             string                       `yaml:"net"`
	MiningAddresses []string                     `yaml:"mining_addresses"`
	EnableCPUMiner  bool                         `yaml:"enable_cpu_miner"`
}

type ShardConfig struct {
	Enable      bool                                    `yaml:"enable"`
	Autorun     bool                                    `yaml:"autorun"`
	ChainParams map[uint32]cprovider.ChainRuntimeConfig `yaml:"chain_params"`
}

func (cfg *NodeConfig) ChainParams() *chaincfg.Params {
	return chaincfg.NetName(cfg.Net).Params()
}

func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
