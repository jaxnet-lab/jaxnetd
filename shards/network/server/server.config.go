package server

import (
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"net"
	"os"
	"time"
)

type P2pConfig struct {
	Peers           []string `yaml:"peers" `
	Listeners       []string `yaml:"listeners"`
	AgentBlacklist  []string `yaml:"agent_blacklist"`
	AgentWhitelist  []string `yaml:"agent_whitelist"`
	DisableListen   bool     `yaml:"disable_listen" long:"nolisten" description:"Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen"`
	SigCacheMaxSize uint     `yaml:"sig_cache_max_size" long:"sigcachemaxsize" description:"The maximum number of entries in the signature verification cache"`
	AddCheckpoints  []string `yaml:"add_checkpoints" long:"addcheckpoint" description:"Add a custom checkpoint.  Format: '<height>:<hash>'"`
	//AddPeers        []string `yaml:"add_peers" short:"a" long:"addpeer" description:"Add a server to connect with at startup"`
	AddrIndex bool `yaml:"addr_index" long:"addrindex" description:"Maintain a full address-based transaction index which makes the searchrawtransactions RPC available"`

	ExternalIPs       []string      `yaml:"external_ips" long:"externalip" description:"Add an ip to the list of local addresses we claim to listen on to peers"`
	ConnectPeers      []string      `yaml:"connect_peers" long:"connect" description:"Connect only to the specified peers at startup"`
	MaxPeers          int           `yaml:"max_peers" long:"maxpeers" description:"Max number of inbound and outbound peers"`
	BanDuration       time.Duration `yaml:"ban_duration" long:"banduration" description:"How long to ban misbehaving peers.  Valid time units are {s, m, h}.  Minimum 1 second"`
	BanThreshold      uint32        `yaml:"ban_threshold" long:"banthreshold" description:"Maximum allowed ban score before disconnecting and banning misbehaving peers."`
	DisableBanning    bool          `yaml:"disable_banning" long:"nobanning" description:"Disable banning of misbehaving peers"`
	BlockMaxSize      uint32        `yaml:"block_max_size" long:"blockmaxsize" description:"Maximum block size in bytes to be used when creating a block"`
	BlockMinSize      uint32        `yaml:"block_min_size" long:"blockminsize" description:"Mininum block size in bytes to be used when creating a block"`
	BlockMaxWeight    uint32        `yaml:"block_max_weight" long:"blockmaxweight" description:"Maximum block weight to be used when creating a block"`
	BlockMinWeight    uint32        `yaml:"block_min_weight" long:"blockminweight" description:"Mininum block weight to be used when creating a block"`
	BlockPrioritySize uint32        `yaml:"block_priority_size" long:"blockprioritysize" description:"Size in bytes for high-priority/low-fee transactions when creating a block"`
	BlocksOnly        bool          `yaml:"blocks_only" long:"blocksonly" description:"Do not accept transactions from remote peers."`

	DisableTLS     bool   `yaml:"disable_tls" long:"notls" description:"Disable TLS for the RPC server -- NOTE: This is only allowed if the RPC server is bound to localhost"`
	OnionProxy     string `yaml:"onion_proxy" long:"onion" description:"Connect to tor hidden services via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	OnionProxyPass string `yaml:"onion_proxy_pass" long:"onionpass" default-mask:"-" description:"Password for onion proxy server"`
	OnionProxyUser string `yaml:"onion_proxy_user" long:"onionuser" description:"Username for onion proxy server"`
	Proxy          string `yaml:"proxy" long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	ProxyPass      string `yaml:"proxy_pass" long:"proxypass" default-mask:"-" description:"Password for proxy server"`
	ProxyUser      string `yaml:"proxy_user" long:"proxyuser" description:"Username for proxy server"`
	TxIndex        bool   `yaml:"tx_index" long:"txindex" description:"Maintain a full hash-based transaction index which makes all transactions available via the getrawtransaction RPC"`

	NoRelayPriority     bool          `yaml:"no_relay_priority" long:"norelaypriority" description:"Do not require free or low-fee transactions to have high priority for relaying"`
	RejectNonStd        bool          `yaml:"reject_non_std" long:"rejectnonstd" description:"Reject non-standard transactions regardless of the default settings for the active network."`
	RejectReplacement   bool          `yaml:"reject_replacement" long:"rejectreplacement" description:"Reject transactions that attempt to replace existing transactions within the mempool through the Replace-By-Fee (RBF) signaling policy."`
	RelayNonStd         bool          `yaml:"relay_non_std" long:"relaynonstd" description:"Relay non-standard transactions regardless of the default settings for the active network."`
	FreeTxRelayLimit    float64       `yaml:"free_tx_relay_limit" long:"limitfreerelay" description:"Limit relay of transactions with no transaction fee to the given amount in thousands of bytes per minute"`
	MaxOrphanTxs        int           `yaml:"max_orphan_txs" long:"maxorphantx" description:"Max number of orphan transactions to keep in memory"`
	MinRelayTxFee       float64       `yaml:"min_relay_tx_fee" long:"minrelaytxfee" description:"The minimum transaction fee in BTC/kB to be considered a non-zero fee."`
	TrickleInterval     time.Duration `yaml:"trickle_interval" long:"trickleinterval" description:"Minimum time between attempts to send new inventory to a connected server"`
	MinRelayTxFeeValues btcutil.Amount
	NoCFilters          bool `yaml:"no_c_filters" long:"nocfilters" description:"Disable committed filtering (CF) support"`

	DisableCheckpoints bool `yaml:"disable_checkpoints" long:"nocheckpoints" description:"Disable built-in checkpoints.  Don't do this unless you know what you're doing."`
	DisableDNSSeed     bool `yaml:"disable_dns_seed" long:"nodnsseed" description:"Disable DNS seeding for peers"`
	NoOnion            bool `yaml:"no_onion" long:"noonion" description:"Disable connecting to tor hidden services"`
	Upnp               bool `yaml:"upnp" long:"upnp" description:"Use UPnP to map our listening port outside of NAT"`

	Oniondial func(string, string, time.Duration) (net.Conn, error)
	Dial      func(string, string, time.Duration) (net.Conn, error)
	Lookup    func(string) ([]net.IP, error)
}

func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}