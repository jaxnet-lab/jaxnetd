// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package node

import (
	"os"

	"gitlab.com/jaxnet/jaxnetd/corelog"
	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx/btcd"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

type Config struct {
	ConfigFile  string `toml:"-" yaml:"-" short:"C" long:"configfile" description:"Path to configuration file"`
	ShowVersion bool   `toml:"-" yaml:"-" short:"V" long:"version" description:"Display version information and exit"`

	Node      InstanceConfig     `toml:"node" yaml:"node"`
	LogConfig corelog.Config     `toml:"log_config" yaml:"log_config" `
	BTCD      btcd.Configuration `toml:"btcd" yaml:"btcd"`

	DataDir       string   `toml:"data_dir" yaml:"data_dir" short:"b" long:"datadir" description:"Directory to store data"`
	LogDir        string   `toml:"log_dir" yaml:"log_dir" long:"logdir" description:"Directory to log output."`
	CPUProfile    string   `toml:"cpu_profile" yaml:"cpu_profile" long:"cpuprofile" description:"Write CPU profile to the specified file"`
	DebugLevel    string   `toml:"debug_level" yaml:"debug_level" short:"d" long:"debuglevel" description:"Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available subsystems"`
	Profile       string   `toml:"profile" yaml:"profile" long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	DropAddrIndex bool     `toml:"-" yaml:"-" long:"dropaddrindex" description:"Deletes the address-based transaction index from the database on start up and then exits."`
	DropCfIndex   bool     `toml:"-" yaml:"-" long:"dropcfindex" description:"Deletes the index used for committed filtering (CF) support from the database on start up and then exits."`
	DropTxIndex   bool     `toml:"-" yaml:"-" long:"droptxindex" description:"Deletes the hash-based transaction index from the database on start up and then exits."`
	TorIsolation  bool     `toml:"tor_isolation" yaml:"tor_isolation" long:"torisolation" description:"Enable Tor stream isolation by randomizing user credentials for each connection."`
	Whitelists    []string `toml:"whitelists" yaml:"whitelists" long:"whitelist" description:"Add an IP network or IP that will not be banned. (eg. 192.168.1.0/24 or ::1)"`

	RefillTxIndex bool `toml:"refill_tx_index" yaml:"refill_tx_index"`
	// NoPeerBloomFilters bool     `yaml:"no_peer_bloom_filters" long:"nopeerbloomfilters" description:"Disable bloom filtering support"`
	// UserAgentComments  []string `yaml:"user_agent_comments" long:"uacomment" description:"Comment to add to the user agent -- See BIP 14 for more information."`
}

type InstanceConfig struct {
	BeaconChain         cprovider.ChainRuntimeConfig `yaml:"beacon_chain" toml:"beacon_chain"`
	RPC                 rpc.Config                   `yaml:"rpc" toml:"rpc"`
	P2P                 p2p.Config                   `yaml:"p2p" toml:"p2p"`
	Shards              ShardConfig                  `yaml:"shards" toml:"shards"`
	DBType              string                       `yaml:"db_type" toml:"db_type" description:"Database backend to use for the Block Chain"`
	Net                 string                       `yaml:"net" toml:"net"`
	EnableCPUMiner      bool                         `yaml:"enable_cpu_miner" toml:"enable_cpu_miner"`
	DumpMMR             bool                         `yaml:"dump_mmr" toml:"dump_mmr"`
	AutominingEnabled   bool                         `yaml:"automining_enabled" toml:"automining_enabled"`
	AutominingThreshold int32                        `yaml:"automining_threshold" toml:"automining_threshold"`
}

type ShardConfig struct {
	Enable        bool     `yaml:"enable" toml:"enable"`
	Autorun       bool     `yaml:"autorun" toml:"autorun"`
	EnabledShards []uint32 `yaml:"enabled_shards" toml:"enabled_shards"`
}

func (cfg *InstanceConfig) ChainParams() *chaincfg.Params {
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
