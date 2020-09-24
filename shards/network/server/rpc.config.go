package server

import (
	"net"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/mempool"
	"gitlab.com/jaxnet/core/shard.core.git/peer"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"gitlab.com/jaxnet/core/shard.core.git/shards/types"
)

// Config is a descriptor containing the RPC Server configuration.
type Config struct {
	ListenerAddresses []string `yaml:"listeners"`
	MaxClients        int      `yaml:"maxclients"`
	User              string   `yaml:"user"`
	Password          string   `yaml:"password"`
	Disable           bool     `yaml:"disable" long:"norpc" description:"Disable built-in RPC Server -- NOTE: The RPC Server is disabled by default if no rpcuser/rpcpass or rpclimituser/rpclimitpass is specified"`
	// RPCCert            string  `yaml:"rpc_cert" long:"rpccert" description:"File containing the certificate file"`
	// RPCKey             string  `yaml:"rpc_key" long:"rpckey" description:"File containing the certificate key"`
	LimitPass         string `yaml:"limit_pass"`
	LimitUser         string `yaml:"limit_user"`
	MaxConcurrentReqs int    `yaml:"rpc_max_concurrent_reqs" long:"rpcmaxconcurrentreqs" description:"Max number of concurrent RPC requests that may be processed concurrently"`
	MaxWebsockets     int    `yaml:"rpc_max_websockets" long:"rpcmaxwebsockets" description:"Max number of RPC websocket connections"`

	// Listeners defines a slice of listeners for which the RPC Server will
	// take ownership of and accept connections.  Since the RPC Server takes
	// ownership of these listeners, they will be closed when the RPC Server
	// is stopped.
	Listeners []net.Listener `yaml:"-"`

	WSEnable bool
}

// SetupRPCListeners returns a slice of listeners that are configured for use
// with the RPC server depending on the configuration settings for listen
// addresses and TLS.
func (cfg *Config) SetupRPCListeners() ([]net.Listener, error) {
	// Setup TLS if not disabled.
	listenFunc := net.Listen
	// if !s.cfg.DisableTLS {
	//	// Generate the TLS cert and key file if both don't already
	//	// exist.
	//	if !fileExists(cfg.RPCKey) && !fileExists(cfg.RPCCert) {
	//		err := genCertPair(cfg.RPCCert, cfg.RPCKey)
	//		if err != nil {
	//			return nil, err
	//		}
	//	}
	//	keypair, err := tls.LoadX509KeyPair(cfg.RPCCert, cfg.RPCKey)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	tlsConfig := tls.Config{
	//		Certificates: []tls.Certificate{keypair},
	//		MinVersion:   tls.VersionTLS12,
	//	}
	//
	//	// Change the standard net.Listen function to the tls one.
	//	listenFunc = func(net string, laddr string) (net.Listener, error) {
	//		return tls.Listen(net, laddr, &tlsConfig)
	//	}
	// }

	netAddrs, err := ParseListeners(cfg.ListenerAddresses)
	if err != nil {
		return nil, err
	}

	cfg.Listeners = make([]net.Listener, 0, len(netAddrs))
	for _, addr := range netAddrs {
		listener, err := listenFunc(addr.Network(), addr.String())
		if err != nil {
			continue
		}
		cfg.Listeners = append(cfg.Listeners, listener)
	}

	return cfg.Listeners, nil
}

// rpcserverPeer represents a peer for use with the RPC server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type rpcserverPeer interface {
	// ToPeer returns the underlying peer instance.
	ToPeer() *peer.Peer

	// IsTxRelayDisabled returns whether or not the peer has disabled
	// transaction relay.
	IsTxRelayDisabled() bool

	// BanScore returns the current integer value that represents how close
	// the peer is to being banned.
	BanScore() uint32

	// FeeFilter returns the requested current minimum fee rate for which
	// transactions should be announced.
	FeeFilter() int64
}

// rpcserverConnManager represents a connection manager for use with the RPC
// server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type rpcserverConnManager interface {
	// Connect adds the provided address as a new outbound peer.  The
	// permanent flag indicates whether or not to make the peer persistent
	// and reconnect if the connection is lost.  Attempting to connect to an
	// already existing peer will return an error.
	Connect(addr string, permanent bool) error

	// RemoveByID removes the peer associated with the provided id from the
	// list of persistent peers.  Attempting to remove an id that does not
	// exist will return an error.
	RemoveByID(id int32) error

	// RemoveByAddr removes the peer associated with the provided address
	// from the list of persistent peers.  Attempting to remove an address
	// that does not exist will return an error.
	RemoveByAddr(addr string) error

	// DisconnectByID disconnects the peer associated with the provided id.
	// This applies to both inbound and outbound peers.  Attempting to
	// remove an id that does not exist will return an error.
	DisconnectByID(id int32) error

	// DisconnectByAddr disconnects the peer associated with the provided
	// address.  This applies to both inbound and outbound peers.
	// Attempting to remove an address that does not exist will return an
	// error.
	DisconnectByAddr(addr string) error

	// ConnectedCount returns the number of currently connected peers.
	ConnectedCount() int32

	// NetTotals returns the sum of all bytes received and sent across the
	// network for all peers.
	NetTotals() (uint64, uint64)

	// ConnectedPeers returns an array consisting of all connected peers.
	ConnectedPeers() []rpcserverPeer

	// PersistentPeers returns an array consisting of all the persistent
	// peers.
	PersistentPeers() []rpcserverPeer

	// BroadcastMessage sends the provided message to all currently
	// connected peers.
	BroadcastMessage(msg wire.Message)

	// AddRebroadcastInventory adds the provided inventory to the list of
	// inventories to be rebroadcast at random intervals until they show up
	// in a block.
	AddRebroadcastInventory(iv *types.InvVect, data interface{})

	// RelayTransactions generates and relays inventory vectors for all of
	// the passed transactions to all connected peers.
	RelayTransactions(txns []*mempool.TxDesc)
}

// rpcserverSyncManager represents a sync manager for use with the RPC server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type rpcserverSyncManager interface {
	// IsCurrent returns whether or not the sync manager believes the BlockChain
	// is current as compared to the rest of the network.
	IsCurrent() bool

	// SubmitBlock submits the provided block to the network after
	// processing it locally.
	SubmitBlock(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error)

	// Pause pauses the sync manager until the returned channel is closed.
	Pause() chan<- struct{}

	// SyncPeerID returns the ID of the peer that is currently the peer being
	// used to sync from or 0 if there is none.
	SyncPeerID() int32

	// LocateHeaders returns the headers of the blocks after the first known
	// block in the provided locators until the provided stop hash or the
	// current tip is reached, up to a max of wire.MaxBlockHeadersPerMsg
	// hashes.
	LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []chain.BlockHeader
}

type ShardManager interface {
	ListShards() map[uint32]string

	EnableShard(shardID uint32) error

	DisableShard(shardID uint32) error

	NewShard(shardID uint32, height int32) error
}
