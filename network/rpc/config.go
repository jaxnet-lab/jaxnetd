package rpc

import (
	"net"

	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
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

	netAddrs, err := p2p.ParseListeners(cfg.ListenerAddresses)
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

type ShardManager interface {
	ListShards() []btcjson.ShardInfo

	EnableShard(shardID uint32) error

	DisableShard(shardID uint32) error
}
