// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/go-socks/socks"
	"github.com/jessevdk/go-flags"
	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/connmgr"
	"gitlab.com/jaxnet/core/shard.core.git/database"
	_ "gitlab.com/jaxnet/core/shard.core.git/database/ffldb"
	"gitlab.com/jaxnet/core/shard.core.git/mempool"
	"gitlab.com/jaxnet/core/shard.core.git/peer"
	"gitlab.com/jaxnet/core/shard.core.git/shards"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincore"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigFilename        = "shard.yaml"
	defaultDataDirname           = "data"
	defaultLogLevel              = "info"
	defaultLogDirname            = "logs"
	defaultLogFilename           = "btcd.log"
	defaultMaxPeers              = 125
	defaultBanDuration           = time.Hour * 24
	defaultBanThreshold          = 100
	defaultConnectTimeout        = time.Second * 30
	defaultMaxRPCClients         = 10
	defaultMaxRPCWebsockets      = 25
	defaultMaxRPCConcurrentReqs  = 20
	defaultDbType                = "ffldb"
	defaultFreeTxRelayLimit      = 15.0
	defaultTrickleInterval       = peer.DefaultTrickleInterval
	defaultBlockMinSize          = 0
	defaultBlockMaxSize          = 750000
	defaultBlockMinWeight        = 0
	defaultBlockMaxWeight        = 3000000
	blockMaxSizeMin              = 1000
	blockMaxSizeMax              = blockchain.MaxBlockBaseSize - 1000
	blockMaxWeightMin            = 4000
	blockMaxWeightMax            = blockchain.MaxBlockWeight - 4000
	defaultGenerate              = false
	defaultMaxOrphanTransactions = 100
	defaultMaxOrphanTxSize       = 100000
	defaultSigCacheMaxSize       = 100000
	sampleConfigFilename         = "sample-btcd.yaml"
	defaultTxIndex               = false
	defaultAddrIndex             = false
)

var (
	defaultHomeDir = btcutil.AppDataDir("btcd", false)
	// defaultConfigFile  =  defaultConfigFilename
	// defaultDataDir     = filepath.Join(defaultHomeDir, defaultDataDirname)
	knownDbTypes = database.SupportedDrivers()
	// defaultRPCKeyFile  = filepath.Join(defaultHomeDir, "rpc.key")
	// defaultRPCCertFile = filepath.Join(defaultHomeDir, "rpc.cert")
	// defaultLogDir      = filepath.Join(defaultHomeDir, defaultLogDirname)
)

// runServiceCommand is only set to a real function on Windows.  It is used
// to parse and execute service commands specified via the -s flag.
var runServiceCommand func(string) error

// minUint32 is a helper function to return the minimum of two uint32s.
// This avoids a math import and the need to cast to floats.
func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

//
// // config defines the configuration options for btcd.
// //
// // See loadConfig for details on the configuration load process.
//
// type rpcConfig struct {
//	Listeners  []string `yaml:"listeners"`
//	MaxClients int      `yaml:"maxclients"`
//	User       string   `yaml:"user"`
//	Password   string   `yaml:"password"`
// }
//
// type chainConfig struct {
//	DbType         string   `yaml:"db_type" long:"dbtype" description:"Database backend to use for the Block Chain"`
//	Peers          []string `yaml:"peers" description:"Add a server to connect with at startup"`
//	Listeners      []string `yaml:"listeners" long:"listen" description:"Add an interface/port to listen for connections (default all interfaces port: 8333, testnet: 18333)"`
//	AgentBlacklist []string `yaml:"agent_blacklist" long:"agentblacklist" description:"A comma separated list of user-agent substrings which will cause btcd to reject any peers whose user-agent contains any of the blacklisted substrings."`
//	AgentWhitelist []string `yaml:"agent_whitelist" long:"agentwhitelist" description:"A comma separated list of user-agent substrings which will cause btcd to require all peers' user-agents to contain one of the whitelisted substrings. The blacklist is applied before the blacklist, and an empty whitelist will allow all agents that do not fail the blacklist."`
// }
//
// type shardConfig struct {
// }
//
// type minerConfig struct {
//	Generate    bool     `yaml:"generate" description:"Generate no blocks"`
//	MiningAddrs []string `yaml:"miningaddr" description:"Add the specified payment address to the list of addresses to use for generated blocks -- At least one address is required if the generate option is set"`
// }
//
// type config struct {
//	Beacon            chainConfig   `yaml:"beacon" long:"beacon" description:"Beacon chain"`
//	Miner             minerConfig   `yaml:"miner" long:"miner" description:"Miner config"`
//	RPC               rpcConfig     `yaml:"rpc"`
//	AddCheckpoints    []string      `yaml:"add_checkpoints" long:"addcheckpoint" description:"Add a custom checkpoint.  Format: '<height>:<hash>'"`
//	AddPeers          []string      `yaml:"add_peers" short:"a" long:"addpeer" description:"Add a server to connect with at startup"`
//	AddrIndex         bool          `yaml:"addr_index" long:"addrindex" description:"Maintain a full address-based transaction index which makes the searchrawtransactions RPC available"`
//	BanDuration       time.Duration `yaml:"ban_duration" long:"banduration" description:"How long to ban misbehaving peers.  Valid time units are {s, m, h}.  Minimum 1 second"`
//	BanThreshold      uint32        `yaml:"ban_threshold" long:"banthreshold" description:"Maximum allowed ban score before disconnecting and banning misbehaving peers."`
//	BlockMaxSize      uint32        `yaml:"block_max_size" long:"blockmaxsize" description:"Maximum block size in bytes to be used when creating a block"`
//	BlockMinSize      uint32        `yaml:"block_min_size" long:"blockminsize" description:"Mininum block size in bytes to be used when creating a block"`
//	BlockMaxWeight    uint32        `yaml:"block_max_weight" long:"blockmaxweight" description:"Maximum block weight to be used when creating a block"`
//	BlockMinWeight    uint32        `yaml:"block_min_weight" long:"blockminweight" description:"Mininum block weight to be used when creating a block"`
//	BlockPrioritySize uint32        `yaml:"block_priority_size" long:"blockprioritysize" description:"Size in bytes for high-priority/low-fee transactions when creating a block"`
//	BlocksOnly        bool          `yaml:"blocks_only" long:"blocksonly" description:"Do not accept transactions from remote peers."`
//	ConfigFile        string        `yaml:"config_file" short:"C" long:"configfile" description:"Path to configuration file"`
//	ConnectPeers      []string      `yaml:"connect_peers" long:"connect" description:"Connect only to the specified peers at startup"`
//	CPUProfile        string        `yaml:"cpu_profile" long:"cpuprofile" description:"Write CPU profile to the specified file"`
//	DataDir           string        `yaml:"data_dir" short:"b" long:"datadir" description:"Directory to store data"`
//
//	DebugLevel    string   `yaml:"debug_level" short:"d" long:"debuglevel" description:"Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available subsystems"`
//	DropAddrIndex bool     `yaml:"drop_addr_index" long:"dropaddrindex" description:"Deletes the address-based transaction index from the database on start up and then exits."`
//	DropCfIndex   bool     `yaml:"drop_cf_index" long:"dropcfindex" description:"Deletes the index used for committed filtering (CF) support from the database on start up and then exits."`
//	DropTxIndex   bool     `yaml:"drop_tx_index" long:"droptxindex" description:"Deletes the hash-based transaction index from the database on start up and then exits."`
//	ExternalIPs   []string `yaml:"external_ips" long:"externalip" description:"Add an ip to the list of local addresses we claim to listen on to peers"`
//	//Generate             bool          `yaml:"generate" long:"generate" description:"Generate (mine) bitcoins using the CPU"`
//	FreeTxRelayLimit   float64 `yaml:"free_tx_relay_limit" long:"limitfreerelay" description:"Limit relay of transactions with no transaction fee to the given amount in thousands of bytes per minute"`
//	LogDir             string  `yaml:"log_dir" long:"logdir" description:"Directory to log output."`
//	MaxOrphanTxs       int     `yaml:"max_orphan_txs" long:"maxorphantx" description:"Max number of orphan transactions to keep in memory"`
//	MaxPeers           int     `yaml:"max_peers" long:"maxpeers" description:"Max number of inbound and outbound peers"`
//	MinRelayTxFee      float64 `yaml:"min_relay_tx_fee" long:"minrelaytxfee" description:"The minimum transaction fee in BTC/kB to be considered a non-zero fee."`
//	DisableBanning     bool    `yaml:"disable_banning" long:"nobanning" description:"Disable banning of misbehaving peers"`
//	NoCFilters         bool    `yaml:"no_c_filters" long:"nocfilters" description:"Disable committed filtering (CF) support"`
//	DisableCheckpoints bool    `yaml:"disable_checkpoints" long:"nocheckpoints" description:"Disable built-in checkpoints.  Don't do this unless you know what you're doing."`
//	DisableDNSSeed     bool    `yaml:"disable_dns_seed" long:"nodnsseed" description:"Disable DNS seeding for peers"`
//	DisableListen      bool    `yaml:"disable_listen" long:"nolisten" description:"Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen"`
//	NoOnion            bool    `yaml:"no_onion" long:"noonion" description:"Disable connecting to tor hidden services"`
//	NoPeerBloomFilters bool    `yaml:"no_peer_bloom_filters" long:"nopeerbloomfilters" description:"Disable bloom filtering support"`
//	NoRelayPriority    bool    `yaml:"no_relay_priority" long:"norelaypriority" description:"Do not require free or low-fee transactions to have high priority for relaying"`
//	DisableRPC         bool    `yaml:"disable_rpc" long:"norpc" description:"Disable built-in RPC server -- NOTE: The RPC server is disabled by default if no rpcuser/rpcpass or rpclimituser/rpclimitpass is specified"`
//	DisableTLS         bool    `yaml:"disable_tls" long:"notls" description:"Disable TLS for the RPC server -- NOTE: This is only allowed if the RPC server is bound to localhost"`
//	OnionProxy         string  `yaml:"onion_proxy" long:"onion" description:"Connect to tor hidden services via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
//	OnionProxyPass     string  `yaml:"onion_proxy_pass" long:"onionpass" default-mask:"-" description:"Password for onion proxy server"`
//	OnionProxyUser     string  `yaml:"onion_proxy_user" long:"onionuser" description:"Username for onion proxy server"`
//	Profile            string  `yaml:"profile" long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
//	Proxy              string  `yaml:"proxy" long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
//	ProxyPass          string  `yaml:"proxy_pass" long:"proxypass" default-mask:"-" description:"Password for proxy server"`
//	ProxyUser          string  `yaml:"proxy_user" long:"proxyuser" description:"Username for proxy server"`
//	RegressionTest     bool    `yaml:"regression_test" long:"regtest" description:"Use the regression test network"`
//	RejectNonStd       bool    `yaml:"reject_non_std" long:"rejectnonstd" description:"Reject non-standard transactions regardless of the default settings for the active network."`
//	RejectReplacement  bool    `yaml:"reject_replacement" long:"rejectreplacement" description:"Reject transactions that attempt to replace existing transactions within the mempool through the Replace-By-Fee (RBF) signaling policy."`
//	RelayNonStd        bool    `yaml:"relay_non_std" long:"relaynonstd" description:"Relay non-standard transactions regardless of the default settings for the active network."`
//	RPCCert            string  `yaml:"rpc_cert" long:"rpccert" description:"File containing the certificate file"`
//	RPCKey             string  `yaml:"rpc_key" long:"rpckey" description:"File containing the certificate key"`
//	RPCLimitPass       string  `yaml:"rpc_limit_pass" long:"rpclimitpass" default-mask:"-" description:"Password for limited RPC connections"`
//	RPCLimitUser       string  `yaml:"rpc_limit_user" long:"rpclimituser" description:"Username for limited RPC connections"`
//	//RPCListeners         []string      `yaml:"rpc_listeners" long:"rpclisten" description:"Add an interface/port to listen for RPC connections (default port: 8334, testnet: 18334)"`
//	//RPCMaxClients        int  `yaml:"rpc_max_clients" long:"rpcmaxclients" description:"Max number of RPC clients for standard connections"`
//	RPCMaxConcurrentReqs int  `yaml:"rpc_max_concurrent_reqs" long:"rpcmaxconcurrentreqs" description:"Max number of concurrent RPC requests that may be processed concurrently"`
//	RPCMaxWebsockets     int  `yaml:"rpc_max_websockets" long:"rpcmaxwebsockets" description:"Max number of RPC websocket connections"`
//	RPCQuirks            bool `yaml:"rpc_quirks" long:"rpcquirks" description:"Mirror some JSON-RPC quirks of Bitcoin Core -- NOTE: Discouraged unless interoperability issues need to be worked around"`
//	//RPCPass              string        `yaml:"rpc_pass" short:"P" long:"rpcpass" default-mask:"-" description:"Password for RPC connections"`
//	//RPCUser              string        `yaml:"rpc_user" short:"u" long:"rpcuser" description:"Username for RPC connections"`
//	SigCacheMaxSize   uint          `yaml:"sig_cache_max_size" long:"sigcachemaxsize" description:"The maximum number of entries in the signature verification cache"`
//	SimNet            bool          `yaml:"simnet" long:"simnet" description:"Use the simulation test network"`
//	TestNet3          bool          `yaml:"testnet" long:"testnet" description:"Use the test network"`
//	TorIsolation      bool          `yaml:"tor_isolation" long:"torisolation" description:"Enable Tor stream isolation by randomizing user credentials for each connection."`
//	TrickleInterval   time.Duration `yaml:"trickle_interval" long:"trickleinterval" description:"Minimum time between attempts to send new inventory to a connected server"`
//	TxIndex           bool          `yaml:"tx_index" long:"txindex" description:"Maintain a full hash-based transaction index which makes all transactions available via the getrawtransaction RPC"`
//	UserAgentComments []string      `yaml:"user_agent_comments" long:"uacomment" description:"Comment to add to the user agent -- See BIP 14 for more information."`
//	Upnp              bool          `yaml:"upnp" long:"upnp" description:"Use UPnP to map our listening port outside of NAT"`
//	ShowVersion       bool          `yaml:"show_version" short:"V" long:"version" description:"Display version information and exit"`
//	Whitelists        []string      `yaml:"whitelists" long:"whitelist" description:"Add an IP network or IP that will not be banned. (eg. 192.168.1.0/24 or ::1)"`
//	lookup            func(string) ([]net.IP, error)
//	oniondial         func(string, string, time.Duration) (net.Conn, error)
//	dial              func(string, string, time.Duration) (net.Conn, error)
//	addCheckpoints    []chaincfg.Checkpoint
//	miningAddrs       []btcutil.Address
//	minRelayTxFee     btcutil.Amount
//	whitelists        []*net.IPNet
// }

// serviceOptions defines the configuration options for the daemon as a service on
// Windows.
type serviceOptions struct {
	ServiceCommand string `short:"s" long:"service" description:"Service command {install, remove, start, stop}"`
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(defaultHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// validLogLevel returns whether or not logLevel is a valid debug log level.
func validLogLevel(logLevel string) bool {
	switch logLevel {
	case "trace":
		fallthrough
	case "debug":
		fallthrough
	case "info":
		fallthrough
	case "warn":
		fallthrough
	case "error":
		fallthrough
	case "critical":
		return true
	}
	return false
}

// supportedSubsystems returns a sorted slice of the supported subsystems for
// logging purposes.
func supportedSubsystems() []string {
	// Convert the subsystemLoggers map keys to a slice.
	subsystems := make([]string, 0, len(subsystemLoggers))
	for subsysID := range subsystemLoggers {
		subsystems = append(subsystems, subsysID)
	}

	// Sort the subsystems for stable display.
	sort.Strings(subsystems)
	return subsystems
}

// parseAndSetDebugLevels attempts to parse the specified debug level and set
// the levels accordingly.  An appropriate error is returned if anything is
// invalid.
func parseAndSetDebugLevels(debugLevel string) error {
	// When the specified string doesn't have any delimters, treat it as
	// the log level for all subsystems.
	if !strings.Contains(debugLevel, ",") && !strings.Contains(debugLevel, "=") {
		// Validate debug log level.
		if !validLogLevel(debugLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, debugLevel)
		}

		// Change the logging level for all subsystems.
		setLogLevels(debugLevel)

		return nil
	}

	// Split the specified string into subsystem/level pairs while detecting
	// issues and update the log levels accordingly.
	for _, logLevelPair := range strings.Split(debugLevel, ",") {
		if !strings.Contains(logLevelPair, "=") {
			str := "The specified debug level contains an invalid " +
				"subsystem/level pair [%v]"
			return fmt.Errorf(str, logLevelPair)
		}

		// Extract the specified subsystem and log level.
		fields := strings.Split(logLevelPair, "=")
		subsysID, logLevel := fields[0], fields[1]

		// Validate subsystem.
		if _, exists := subsystemLoggers[subsysID]; !exists {
			str := "The specified subsystem [%v] is invalid -- " +
				"supported subsytems %v"
			return fmt.Errorf(str, subsysID, supportedSubsystems())
		}

		// Validate log level.
		if !validLogLevel(logLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, logLevel)
		}

		setLogLevel(subsysID, logLevel)
	}

	return nil
}

// validDbType returns whether or not dbType is a supported database type.
func validDbType(dbType string) bool {
	for _, knownType := range knownDbTypes {
		if dbType == knownType {
			return true
		}
	}

	return false
}

// removeDuplicateAddresses returns a new slice with all duplicate entries in
// addrs removed.
func removeDuplicateAddresses(addrs []string) []string {
	result := make([]string, 0, len(addrs))
	seen := map[string]struct{}{}
	for _, val := range addrs {
		if _, ok := seen[val]; !ok {
			result = append(result, val)
			seen[val] = struct{}{}
		}
	}
	return result
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

// normalizeAddresses returns a new slice with all the passed server addresses
// normalized with the given default port, and all duplicates removed.
func normalizeAddresses(addrs []string, defaultPort string) []string {
	for i, addr := range addrs {
		addrs[i] = normalizeAddress(addr, defaultPort)
	}

	return removeDuplicateAddresses(addrs)
}

// newCheckpointFromStr parses checkpoints in the '<height>:<hash>' format.
func newCheckpointFromStr(checkpoint string) (chaincore.Checkpoint, error) {
	parts := strings.Split(checkpoint, ":")
	if len(parts) != 2 {
		return chaincore.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q -- use the syntax <height>:<hash>",
			checkpoint)
	}

	height, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return chaincore.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q due to malformed height", checkpoint)
	}

	if len(parts[1]) == 0 {
		return chaincore.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q due to missing hash", checkpoint)
	}
	hash, err := chainhash.NewHashFromStr(parts[1])
	if err != nil {
		return chaincore.Checkpoint{}, fmt.Errorf("unable to parse "+
			"checkpoint %q due to malformed hash", checkpoint)
	}

	return chaincore.Checkpoint{
		Height: int32(height),
		Hash:   hash,
	}, nil
}

// parseCheckpoints checks the checkpoint strings for valid syntax
// ('<height>:<hash>') and parses them to chaincfg.Checkpoint instances.
func parseCheckpoints(checkpointStrings []string) ([]chaincore.Checkpoint, error) {
	if len(checkpointStrings) == 0 {
		return nil, nil
	}
	checkpoints := make([]chaincore.Checkpoint, len(checkpointStrings))
	for i, cpString := range checkpointStrings {
		checkpoint, err := newCheckpointFromStr(cpString)
		if err != nil {
			return nil, err
		}
		checkpoints[i] = checkpoint
	}
	return checkpoints, nil
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// newConfigParser returns a new command line flags parser.
func newConfigParser(cfg *shards.Config, so *serviceOptions, options flags.Options) *flags.Parser {
	parser := flags.NewParser(cfg, options)
	if runtime.GOOS == "windows" {
		parser.AddGroup("Service Options", "Service Options", so)
	}
	return parser
}

// loadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
// 	1) Start with a default config with sane settings
// 	2) Pre-parse the command line to check for an alternative config file
// 	3) Load configuration file overwriting defaults with any specified options
// 	4) Parse CLI options and overwrite/add any specified options
//
// The above results in btcd functioning properly without any config settings
// while still allowing the user to override settings with config files and
// command line options.  Command line options always take precedence.
func loadConfig() (*shards.Config, []string, error) {

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = defaultHomeDir
	}

	configFile := path.Join(dataDir, defaultConfigFilename)
	// Default config.
	fileName := os.Getenv("configName")
	if fileName != "" {
		configFile = filepath.Join(dataDir, fileName)
	}

	cfg := shards.Config{
		Node: shards.NodeConfig{
			Net:    activeNetParams.Name,
			DbType: defaultDbType,
			BeaconChain: server.ChainRuntimeConfig{
				BlockMinSize:      defaultBlockMinSize,
				BlockMaxSize:      defaultBlockMaxSize,
				BlockMinWeight:    defaultBlockMinWeight,
				BlockMaxWeight:    defaultBlockMaxWeight,
				BlockPrioritySize: mempool.DefaultBlockPrioritySize,
				MaxPeers:          defaultMaxPeers,
				MinRelayTxFee:     mempool.DefaultMinRelayTxFee.ToBTC(),
				FreeTxRelayLimit:  defaultFreeTxRelayLimit,
				TxIndex:           defaultTxIndex,
				AddrIndex:         defaultAddrIndex,
				MaxOrphanTxs:      defaultMaxOrphanTransactions,
				SigCacheMaxSize:   defaultSigCacheMaxSize,
			},
			P2P: server.P2pConfig{
				BanThreshold:    defaultBanThreshold,
				BanDuration:     defaultBanDuration,
				TrickleInterval: defaultTrickleInterval,
			},
			RPC: server.Config{
				MaxClients: defaultMaxRPCClients,
				// RPCMaxWebsockets:     defaultMaxRPCWebsockets,
				// RPCMaxConcurrentReqs: defaultMaxRPCConcurrentReqs,
			},
		},

		ConfigFile: configFile,
		DebugLevel: defaultLogLevel,
		DataDir:    dataDir,
		LogDir:     path.Join(dataDir, "logs"),
		// RPCKey:               defaultRPCKeyFile,
		// RPCCert:              defaultRPCCertFile,
		// Generate:             defaultGenerate,
	}

	// Service options which are only added on Windows.
	serviceOpts := serviceOptions{}

	// Pre-parse the command line options to see if an alternative config
	// file or the version flag was specified.  Any errors aside from the
	// help message error can be ignored here since they will be caught by
	// the final parse below.
	preCfg := cfg
	preParser := newConfigParser(&preCfg, &serviceOpts, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			return nil, nil, err
		}
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)
	if preCfg.ShowVersion {
		os.Exit(0)
	}

	// Perform service command and exit if specified.  Invalid service
	// commands show an appropriate error.  Only runs on Windows since
	// the runServiceCommand function will be nil when not on Windows.
	if serviceOpts.ServiceCommand != "" && runServiceCommand != nil {
		err := runServiceCommand(serviceOpts.ServiceCommand)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(0)
	}

	// Load additional config from file.
	var configFileError error
	parser := newConfigParser(&cfg, &serviceOpts, flags.Default)

	stats, err := os.Stat(preCfg.ConfigFile)
	if os.IsNotExist(err) {
		err := createDefaultConfigFile(preCfg.ConfigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating a "+
				"default config file: %v\n", err)
		}
	}

	cfgFile, err := os.OpenFile(preCfg.ConfigFile, os.O_RDONLY, 0644)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
		_, _ = fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	ext := filepath.Ext(stats.Name())
	switch ext {
	case ".conf":
		err = flags.NewIniParser(parser).Parse(cfgFile)
		if err != nil {
			configFileError = err
		}
	case ".yaml":
		err = yaml.NewDecoder(cfgFile).Decode(&cfg)
		if err != nil {
			configFileError = err
		}
	default:
		_, _ = fmt.Fprintln(os.Stderr, "Invalid file extension:", ext)
		return nil, nil, errors.New("Invalid file extension: " + ext)
	}

	// Parse command line options again to ensure they take precedence.
	remainingArgs, err := parser.Parse()
	if err != nil {
		fmt.Println(err)
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, nil, err
	}

	// Create the home directory if it doesn't already exist.
	funcName := "loadConfig"
	err = os.MkdirAll(dataDir, 0700)
	if err != nil {
		// Show a nicer error message if it's because a symlink is
		// linked to a directory that does not exist (probably because
		// it's not mounted).
		if e, ok := err.(*os.PathError); ok && os.IsExist(err) {
			if link, lerr := os.Readlink(e.Path); lerr == nil {
				str := "is symlink %s -> %s mounted?"
				err = fmt.Errorf(str, e.Path, link)
			}
		}

		str := "%s: Failed to create home directory: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	activeNetParams.Params = cfg.Node.ChainParams()

	// Set the default policy for relaying non-standard transactions
	// according to the default of the active network. The set
	// configuration value takes precedence over the default value for the
	// selected network.
	relayNonStd := activeNetParams.RelayNonStdTxs
	cfg.Node.BeaconChain.RelayNonStd = relayNonStd

	// Append the network type to the data directory so it is "namespaced"
	// per network.  In addition to the block database, there are other
	// pieces of data that are saved to disk such as address manager state.
	// All data is specific to a network, so namespacing the data directory
	// means each individual piece of serialized data does not have to
	// worry about changing names per network and such.
	cfg.DataDir = cleanAndExpandPath(cfg.DataDir)
	cfg.DataDir = filepath.Join(cfg.DataDir, netName(activeNetParams))

	// Append the network type to the log directory so it is "namespaced"
	// per network in the same fashion as the data directory.
	cfg.LogDir = cleanAndExpandPath(cfg.LogDir)
	cfg.LogDir = filepath.Join(cfg.LogDir, netName(activeNetParams))

	// Special show command to list supported subsystems and exit.
	if cfg.DebugLevel == "show" {
		fmt.Println("Supported subsystems", supportedSubsystems())
		os.Exit(0)
	}

	// Initialize log rotation.  After log rotation has been initialized, the
	// logger variables may be used.
	initLogRotator(filepath.Join(cfg.LogDir, defaultLogFilename))

	// Parse, validate, and set debug log level(s).
	if err := parseAndSetDebugLevels(cfg.DebugLevel); err != nil {
		err := fmt.Errorf("%s: %v", funcName, err.Error())
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	if cfg.Node.ChainParams() == nil {
		str := "%s: The specified net name [%v] is invalid"
		err := fmt.Errorf(str, funcName, cfg.Node.Net)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Validate database type.
	if !validDbType(cfg.Node.DbType) {
		str := "%s: The specified database type [%v] is invalid -- " +
			"supported types %v"
		err := fmt.Errorf(str, funcName, cfg.Node.DbType, knownDbTypes)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Validate profile port number
	if cfg.Profile != "" {
		profilePort, err := strconv.Atoi(cfg.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			str := "%s: The profile port must be between 1024 and 65535"
			err := fmt.Errorf(str, funcName)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
	}

	// Don't allow ban durations that are too short.
	if cfg.Node.P2P.BanDuration < time.Second {
		str := "%s: The banduration option may not be less than 1s -- parsed [%v]"
		err := fmt.Errorf(str, funcName, cfg.Node.P2P.BanDuration)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Validate any given whitelisted IP addresses and networks.
	// if len(cfg.Whitelists) > 0 {
	//	var ip net.IP
	//	cfg.whitelists = make([]*net.IPNet, 0, len(cfg.Whitelists))
	//
	//	for _, addr := range cfg.Whitelists {
	//		_, ipnet, err := net.ParseCIDR(addr)
	//		if err != nil {
	//			ip = net.ParseIP(addr)
	//			if ip == nil {
	//				str := "%s: The whitelist value of '%s' is invalid"
	//				err = fmt.Errorf(str, funcName, addr)
	//				fmt.Fprintln(os.Stderr, err)
	//				fmt.Fprintln(os.Stderr, usageMessage)
	//				return nil, nil, err
	//			}
	//			var bits int
	//			if ip.To4() == nil {
	//				// IPv6
	//				bits = 128
	//			} else {
	//				bits = 32
	//			}
	//			ipnet = &net.IPNet{
	//				IP:   ip,
	//				Mask: net.CIDRMask(bits, bits),
	//			}
	//		}
	//		cfg.whitelists = append(cfg.whitelists, ipnet)
	//	}
	// }

	// --addPeer and --connect do not mix.
	if len(cfg.Node.P2P.Peers) > 0 && len(cfg.Node.P2P.ConnectPeers) > 0 {
		str := "%s: the --addpeer and --connect options can not be " +
			"mixed"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// --proxy or --connect without --listen disables listening.
	if (cfg.Node.P2P.Proxy != "" || len(cfg.Node.P2P.ConnectPeers) > 0) &&
		len(cfg.Node.P2P.Listeners) == 0 {
		cfg.Node.P2P.DisableListen = true
	}

	// Connect means no DNS seeding.
	if len(cfg.Node.P2P.ConnectPeers) > 0 {
		cfg.Node.P2P.DisableDNSSeed = true
	}

	// Add the default listener if none were specified. The default
	// listener is all addresses on the listen port for the network
	// we are to connect to.
	if len(cfg.Node.P2P.Listeners) == 0 {
		cfg.Node.P2P.Listeners = []string{
			net.JoinHostPort("", activeNetParams.DefaultPort),
		}
	}

	// Check to make sure limited and admin users don't have the same username
	if cfg.Node.RPC.User == cfg.Node.RPC.LimitUser && cfg.Node.RPC.User != "" {
		str := "%s: --rpcuser and --rpclimituser must not specify the " +
			"same username"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Check to make sure limited and admin users don't have the same password
	if cfg.Node.RPC.Password == cfg.Node.RPC.LimitPass && cfg.Node.RPC.Password != "" {
		str := "%s: --rpcpass and --rpclimitpass must not specify the " +
			"same password"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// The RPC server is disabled if no username or password is provided.
	if (cfg.Node.RPC.User == "" || cfg.Node.RPC.Password == "") &&
		(cfg.Node.RPC.LimitUser == "" || cfg.Node.RPC.LimitPass == "") {
		cfg.Node.RPC.Disable = true
	}

	if cfg.Node.RPC.Disable {
		btcdLog.Infof("RPC service is disabled")
	}

	// Default RPC to listen on localhost only.
	if !cfg.Node.RPC.Disable && len(cfg.Node.RPC.ListenerAddresses) == 0 {
		addrs, err := net.LookupHost("localhost")
		if err != nil {
			return nil, nil, err
		}
		cfg.Node.RPC.ListenerAddresses = make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addr = net.JoinHostPort(addr, activeNetParams.rpcPort)
			cfg.Node.RPC.ListenerAddresses = append(cfg.Node.RPC.ListenerAddresses, addr)
		}

	}

	_, err = cfg.Node.RPC.SetupRPCListeners()
	if err != nil {
		return nil, nil, err
	}

	if cfg.Node.RPC.MaxConcurrentReqs < 0 {
		str := "%s: The rpcmaxwebsocketconcurrentrequests option may " +
			"not be less than 0 -- parsed [%d]"
		err := fmt.Errorf(str, funcName, cfg.Node.RPC.MaxConcurrentReqs)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Validate the the minrelaytxfee.
	cfg.Node.BeaconChain.MinRelayTxFeeValues, err = btcutil.NewAmount(cfg.Node.BeaconChain.MinRelayTxFee)
	if err != nil {
		str := "%s: invalid minrelaytxfee: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Limit the max block size to a sane value.
	if cfg.Node.BeaconChain.BlockMaxSize < blockMaxSizeMin || cfg.Node.BeaconChain.BlockMaxSize >
		blockMaxSizeMax {

		str := "%s: The blockmaxsize option must be in between %d " +
			"and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, blockMaxSizeMin,
			blockMaxSizeMax, cfg.Node.BeaconChain.BlockMaxSize)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Limit the max block weight to a sane value.
	if cfg.Node.BeaconChain.BlockMaxWeight < blockMaxWeightMin ||
		cfg.Node.BeaconChain.BlockMaxWeight > blockMaxWeightMax {

		str := "%s: The blockmaxweight option must be in between %d " +
			"and %d -- parsed [%d]"
		err := fmt.Errorf(str, funcName, blockMaxWeightMin,
			blockMaxWeightMax, cfg.Node.BeaconChain.BlockMaxWeight)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Limit the max orphan count to a sane vlue.
	if cfg.Node.BeaconChain.MaxOrphanTxs < 0 {
		str := "%s: The maxorphantx option may not be less than 0 " +
			"-- parsed [%d]"
		err := fmt.Errorf(str, funcName, cfg.Node.BeaconChain.MaxOrphanTxs)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Limit the block priority and minimum block sizes to max block size.
	cfg.Node.BeaconChain.BlockPrioritySize = minUint32(cfg.Node.BeaconChain.BlockPrioritySize, cfg.Node.BeaconChain.BlockMaxSize)
	cfg.Node.BeaconChain.BlockMinSize = minUint32(cfg.Node.BeaconChain.BlockMinSize, cfg.Node.BeaconChain.BlockMaxSize)
	cfg.Node.BeaconChain.BlockMinWeight = minUint32(cfg.Node.BeaconChain.BlockMinWeight, cfg.Node.BeaconChain.BlockMaxWeight)

	switch {
	// If the max block size isn't set, but the max weight is, then we'll
	// set the limit for the max block size to a safe limit so weight takes
	// precedence.
	case cfg.Node.BeaconChain.BlockMaxSize == defaultBlockMaxSize &&
		cfg.Node.BeaconChain.BlockMaxWeight != defaultBlockMaxWeight:

		cfg.Node.BeaconChain.BlockMaxSize = blockchain.MaxBlockBaseSize - 1000

	// If the max block weight isn't set, but the block size is, then we'll
	// scale the set weight accordingly based on the max block size value.
	case cfg.Node.BeaconChain.BlockMaxSize != defaultBlockMaxSize &&
		cfg.Node.BeaconChain.BlockMaxWeight == defaultBlockMaxWeight:

		cfg.Node.BeaconChain.BlockMaxWeight = cfg.Node.BeaconChain.BlockMaxSize * blockchain.WitnessScaleFactor
	}

	// // Look for illegal characters in the user agent comments.
	// for _, uaComment := range cfg.Node.P2P.UserAgentComments {
	//	if strings.ContainsAny(uaComment, "/:()") {
	//		err := fmt.Errorf("%s: The following characters must not "+
	//			"appear in user agent comments: '/', ':', '(', ')'",
	//			funcName)
	//		fmt.Fprintln(os.Stderr, err)
	//		fmt.Fprintln(os.Stderr, usageMessage)
	//		return nil, nil, err
	//	}
	// }

	// --txindex and --droptxindex do not mix.
	if cfg.Node.BeaconChain.TxIndex && cfg.DropTxIndex {
		err := fmt.Errorf("%s: the --txindex and --droptxindex "+
			"options may  not be activated at the same time",
			funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// --addrindex and --dropaddrindex do not mix.
	if cfg.Node.BeaconChain.AddrIndex && cfg.DropAddrIndex {
		err := fmt.Errorf("%s: the --addrindex and --dropaddrindex "+
			"options may not be activated at the same time",
			funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// --addrindex and --droptxindex do not mix.
	if cfg.Node.BeaconChain.AddrIndex && cfg.DropTxIndex {
		err := fmt.Errorf("%s: the --addrindex and --droptxindex "+
			"options may not be activated at the same time "+
			"because the address index relies on the transaction "+
			"index",
			funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Add default port to all listener addresses if needed and remove
	// duplicate addresses.
	cfg.Node.P2P.Listeners = normalizeAddresses(cfg.Node.P2P.Listeners,
		activeNetParams.DefaultPort)

	// Add default port to all rpc listener addresses if needed and remove
	// duplicate addresses.
	cfg.Node.RPC.ListenerAddresses = normalizeAddresses(cfg.Node.RPC.ListenerAddresses,
		activeNetParams.rpcPort)

	// Only allow TLS to be disabled if the RPC is bound to localhost
	// addresses.
	if !cfg.Node.RPC.Disable && cfg.Node.P2P.DisableTLS {
		allowedTLSListeners := map[string]struct{}{
			"localhost": {},
			"127.0.0.1": {},
			"0.0.0.0":   {}, // TODO: setup tls
			"::1":       {},
		}
		for _, addr := range cfg.Node.RPC.ListenerAddresses {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				str := "%s: RPC listen interface '%s' is " +
					"invalid: %v"
				err := fmt.Errorf(str, funcName, addr, err)
				fmt.Fprintln(os.Stderr, err)
				fmt.Fprintln(os.Stderr, usageMessage)
				return nil, nil, err
			}
			if _, ok := allowedTLSListeners[host]; !ok {
				str := "%s: the --notls option may not be used " +
					"when binding RPC to non localhost " +
					"addresses: %s"
				err := fmt.Errorf(str, funcName, addr)
				fmt.Fprintln(os.Stderr, err)
				fmt.Fprintln(os.Stderr, usageMessage)
				return nil, nil, err
			}
		}
	}

	// Add default port to all added server addresses if needed and remove
	// duplicate addresses.
	cfg.Node.P2P.Peers = normalizeAddresses(cfg.Node.P2P.Peers,
		activeNetParams.DefaultPort)
	cfg.Node.P2P.ConnectPeers = normalizeAddresses(cfg.Node.P2P.ConnectPeers,
		activeNetParams.DefaultPort)

	// --noonion and --onion do not mix.
	if cfg.Node.P2P.NoOnion && cfg.Node.P2P.OnionProxy != "" {
		err := fmt.Errorf("%s: the --noonion and --onion options may "+
			"not be activated at the same time", funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Check the checkpoints for syntax errors.
	// cfg.addCheckpoints, err = parseCheckpoints(cfg.AddCheckpoints)
	// if err != nil {
	//	str := "%s: Error parsing checkpoints: %v"
	//	err := fmt.Errorf(str, funcName, err)
	//	fmt.Fprintln(os.Stderr, err)
	//	fmt.Fprintln(os.Stderr, usageMessage)
	//	return nil, nil, err
	// }

	// Tor stream isolation requires either proxy or onion proxy to be set.
	if cfg.TorIsolation && cfg.Node.P2P.Proxy == "" && cfg.Node.P2P.OnionProxy == "" {
		str := "%s: Tor stream isolation requires either proxy or " +
			"onionproxy to be set"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Setup dial and DNS resolution (lookup) functions depending on the
	// specified options.  The default is to use the standard
	// net.DialTimeout function as well as the system DNS resolver.  When a
	// proxy is specified, the dial function is set to the proxy specific
	// dial function and the lookup is set to use tor (unless --noonion is
	// specified in which case the system DNS resolver is used).
	cfg.Node.P2P.Dial = net.DialTimeout
	cfg.Node.P2P.Lookup = net.LookupIP
	if cfg.Node.P2P.Proxy != "" {
		_, _, err := net.SplitHostPort(cfg.Node.P2P.Proxy)
		if err != nil {
			str := "%s: Proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, cfg.Node.P2P.Proxy, err)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}

		// Tor isolation flag means proxy credentials will be overridden
		// unless there is also an onion proxy configured in which case
		// that one will be overridden.
		torIsolation := false
		if cfg.TorIsolation && cfg.Node.P2P.OnionProxy == "" &&
			(cfg.Node.P2P.ProxyUser != "" || cfg.Node.P2P.ProxyPass != "") {

			torIsolation = true
			fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
				"overriding specified proxy user credentials")
		}

		proxy := &socks.Proxy{
			Addr:         cfg.Node.P2P.Proxy,
			Username:     cfg.Node.P2P.ProxyUser,
			Password:     cfg.Node.P2P.ProxyPass,
			TorIsolation: torIsolation,
		}
		cfg.Node.P2P.Dial = proxy.DialTimeout

		// Treat the proxy as tor and perform DNS resolution through it
		// unless the --noonion flag is set or there is an
		// onion-specific proxy configured.
		if !cfg.Node.P2P.NoOnion && cfg.Node.P2P.OnionProxy == "" {
			cfg.Node.P2P.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, cfg.Node.P2P.Proxy)
			}
		}
	}

	// Setup onion address dial function depending on the specified options.
	// The default is to use the same dial function selected above.  However,
	// when an onion-specific proxy is specified, the onion address dial
	// function is set to use the onion-specific proxy while leaving the
	// normal dial function as selected above.  This allows .onion address
	// traffic to be routed through a different proxy than normal traffic.
	if cfg.Node.P2P.OnionProxy != "" {
		_, _, err := net.SplitHostPort(cfg.Node.P2P.OnionProxy)
		if err != nil {
			str := "%s: Onion proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, cfg.Node.P2P.OnionProxy, err)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}

		// Tor isolation flag means onion proxy credentials will be
		// overridden.
		if cfg.TorIsolation &&
			(cfg.Node.P2P.OnionProxyUser != "" || cfg.Node.P2P.OnionProxyPass != "") {
			fmt.Fprintln(os.Stderr, "Tor isolation set -- "+
				"overriding specified onionproxy user "+
				"credentials ")
		}

		cfg.Node.P2P.Oniondial = func(network, addr string, timeout time.Duration) (net.Conn, error) {
			proxy := &socks.Proxy{
				Addr:         cfg.Node.P2P.OnionProxy,
				Username:     cfg.Node.P2P.OnionProxyUser,
				Password:     cfg.Node.P2P.OnionProxyPass,
				TorIsolation: cfg.TorIsolation,
			}
			return proxy.DialTimeout(network, addr, timeout)
		}

		// When configured in bridge mode (both --onion and --proxy are
		// configured), it means that the proxy configured by --proxy is
		// not a tor proxy, so override the DNS resolution to use the
		// onion-specific proxy.
		if cfg.Node.P2P.Proxy != "" {
			cfg.Node.P2P.Lookup = func(host string) ([]net.IP, error) {
				return connmgr.TorLookupIP(host, cfg.Node.P2P.OnionProxy)
			}
		}
	} else {
		cfg.Node.P2P.Oniondial = cfg.Node.P2P.Dial
	}

	// Specifying --noonion means the onion address dial function results in
	// an error.
	if cfg.Node.P2P.NoOnion {
		cfg.Node.P2P.Oniondial = func(a, b string, t time.Duration) (net.Conn, error) {
			return nil, errors.New("tor has been disabled")
		}
	}

	// Warn about missing config file only after all other configuration is
	// done.  This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		btcdLog.Warnf("%v", configFileError)
	}

	return &cfg, remainingArgs, nil
}

// createDefaultConfig copies the file sample-btcd.conf to the given destination path,
// and populates it with some randomly generated RPC username and password.
func createDefaultConfigFile(destinationPath string) error {
	// Create the destination directory if it does not exists
	err := os.MkdirAll(filepath.Dir(destinationPath), 0700)
	if err != nil {
		return err
	}

	// We assume sample config file path is same as binary
	path, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	sampleConfigPath := filepath.Join(path, sampleConfigFilename)

	// We generate a random user and password
	randomBytes := make([]byte, 20)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}
	generatedRPCUser := base64.StdEncoding.EncodeToString(randomBytes)

	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}
	generatedRPCPass := base64.StdEncoding.EncodeToString(randomBytes)

	src, err := os.Open(sampleConfigPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dest, err := os.OpenFile(destinationPath,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dest.Close()

	// We copy every line from the sample config file to the destination,
	// only replacing the two lines for rpcuser and rpcpass
	reader := bufio.NewReader(src)
	for err != io.EOF {
		var line string
		line, err = reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}

		if strings.Contains(line, "rpcuser=") {
			line = "rpcuser=" + generatedRPCUser + "\n"
		} else if strings.Contains(line, "rpcpass=") {
			line = "rpcpass=" + generatedRPCPass + "\n"
		}

		if _, err := dest.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}
