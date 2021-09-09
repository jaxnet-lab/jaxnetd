// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"net"

	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

// Config is a descriptor containing the RPC Server configuration.
type Config struct {
	ListenerAddresses []string `yaml:"listeners" toml:"listeners"`
	MaxClients        int      `yaml:"maxclients" toml:"maxclients"`
	User              string   `yaml:"user" toml:"user"`
	Password          string   `yaml:"password" toml:"password"`
	Disable           bool     `yaml:"disable" toml:"disable" long:"norpc" description:"Disable built-in RPC Server -- NOTE: The RPC Server is disabled by default if no rpcuser/rpcpass or rpclimituser/rpclimitpass is specified"`
	RPCCert           string   `yaml:"rpc_cert" toml:"rpc_cert" long:"rpccert" description:"File containing the certificate file"`
	RPCKey            string   `yaml:"rpc_key" toml:"rpc_key" long:"rpckey" description:"File containing the certificate key"`
	LimitPass         string   `yaml:"limit_pass" toml:"limit_pass"`
	LimitUser         string   `yaml:"limit_user" toml:"limit_user"`
	MaxConcurrentReqs int      `yaml:"rpc_max_concurrent_reqs" toml:"rpc_max_concurrent_reqs" long:"rpcmaxconcurrentreqs" description:"Max number of concurrent RPC requests that may be processed concurrently"`
	MaxWebsockets     int      `yaml:"rpc_max_websockets" toml:"rpc_max_websockets" long:"rpcmaxwebsockets" description:"Max number of RPC websocket connections"`
	WSEnable          bool     `yaml:"ws_enable" toml:"ws_enable"`

	// Listeners defines a slice of listeners for which the RPC Server will
	// take ownership of and accept connections.  Since the RPC Server takes
	// ownership of these listeners, they will be closed when the RPC Server
	// is stopped.
	Listeners []net.Listener `toml:"-" yaml:"-"`
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
		listener, lErr := listenFunc(addr.Network(), addr.String())
		if lErr != nil {
			err = lErr
			continue
		}
		cfg.Listeners = append(cfg.Listeners, listener)
	}
	if len(cfg.Listeners) < len(netAddrs) {
		return nil, err
	}

	return cfg.Listeners, nil
}

type ShardManager interface {
	ListShards() jaxjson.ShardListResult

	ShardCtl(id uint32) (*cprovider.ChainProvider, bool)

	EnableShard(shardID uint32) error

	DisableShard(shardID uint32) error
}
