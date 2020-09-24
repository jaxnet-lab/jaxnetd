package shards

import (
	"os"

	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
)

// config defines the configuration options for btcd.
//
// See loadConfig for details on the configuration load process.

// type RpcConfig struct {
//	Listeners  []string `yaml:"listeners"`
//	MaxClients int      `yaml:"maxclients"`
//	User       string   `yaml:"user"`
//	Password   string   `yaml:"password"`
// }

// type ChainConfig struct {
//	p2pServer.P2pConfig
// }

type ShardConfig struct {
	Enable bool                `yaml:"enable"`
	IDs    map[uint32]struct{} `yaml:"ids"`
}

type NodeConfig struct {
	RPC             server.Config    `yaml:"rpc"`
	P2P             server.P2pConfig `yaml:"p2p"`
	Shards          ShardConfig      `yaml:"shards"`
	DbType          string           `yaml:"db_type" long:"dbtype" description:"Database backend to use for the Block Chain"`
	Net             string           `yaml:"net"`
	MiningAddresses []string         `yaml:"mining_addresses"`
}

func (cfg *NodeConfig) ChainParams() *chain.Params {
	return chain.NetName(cfg.Net).Params()
}

func (cfg *NodeConfig) ParseMiningAddresses() ([]btcutil.Address, error) {
	params := cfg.ChainParams()
	miningAddrs := make([]btcutil.Address, 0, len(cfg.MiningAddresses))
	for _, address := range cfg.MiningAddresses {
		addr, err := btcutil.DecodeAddress(address, params)
		if err != nil {
			return nil, err
		}

		miningAddrs = append(miningAddrs, addr)
	}
	return miningAddrs, nil
}

type Config struct {
	Node NodeConfig `yaml:"node"`

	ConfigFile    string `yaml:"config_file" short:"C" long:"configfile" description:"Path to configuration file"`
	CPUProfile    string `yaml:"cpu_profile" long:"cpuprofile" description:"Write CPU profile to the specified file"`
	DataDir       string `yaml:"data_dir" short:"b" long:"datadir" description:"Directory to store data"`
	DebugLevel    string `yaml:"debug_level" short:"d" long:"debuglevel" description:"Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available subsystems"`
	DropAddrIndex bool   `yaml:"drop_addr_index" long:"dropaddrindex" description:"Deletes the address-based transaction index from the database on start up and then exits."`
	DropCfIndex   bool   `yaml:"drop_cf_index" long:"dropcfindex" description:"Deletes the index used for committed filtering (CF) support from the database on start up and then exits."`
	DropTxIndex   bool   `yaml:"drop_tx_index" long:"droptxindex" description:"Deletes the hash-based transaction index from the database on start up and then exits."`
	// Generate             bool          `yaml:"generate" long:"generate" description:"Generate (mine) bitcoins using the CPU"`
	LogDir string `yaml:"log_dir" long:"logdir" description:"Directory to log output."`
	// NoPeerBloomFilters bool    `yaml:"no_peer_bloom_filters" long:"nopeerbloomfilters" description:"Disable bloom filtering support"`
	Profile string `yaml:"profile" long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	// RegressionTest     bool    `yaml:"regression_test" long:"regtest" description:"Use the regression test network"`
	TorIsolation bool `yaml:"tor_isolation" long:"torisolation" description:"Enable Tor stream isolation by randomizing user credentials for each connection."`
	// UserAgentComments []string      `yaml:"user_agent_comments" long:"uacomment" description:"Comment to add to the user agent -- See BIP 14 for more information."`
	ShowVersion bool     `yaml:"show_version" short:"V" long:"version" description:"Display version information and exit"`
	Whitelists  []string `yaml:"whitelists" long:"whitelist" description:"Add an IP network or IP that will not be banned. (eg. 192.168.1.0/24 or ::1)"`
	// addCheckpoints    []chaincfg.Checkpoint
	// MiningAddrs       []btcutil.Address
	// whitelists        []*net.IPNet
}

func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
