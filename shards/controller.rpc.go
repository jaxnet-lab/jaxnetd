package shards

import (
	"context"
	"net"

	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
)

func (chainCtl *chainController) runRpc(ctx context.Context, cfg *Config, nodeActor *server.NodeActor) error {
	srv, err := server.RpcServer(&cfg.Node.RPC, nodeActor, chainCtl.logger)
	if err != nil {
		return err
	}

	srv.Start(ctx)
	return nil
}

// setupRPCListeners returns a slice of listeners that are configured for use
// with the RPC server depending on the configuration settings for listen
// addresses and TLS.
func setupRPCListeners(listenerAddr []string) ([]net.Listener, error) {
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

	netAddrs, err := server.ParseListeners(listenerAddr)
	if err != nil {
		return nil, err
	}

	listeners := make([]net.Listener, 0, len(netAddrs))
	for _, addr := range netAddrs {
		listener, err := listenFunc(addr.Network(), addr.String())
		if err != nil {
			continue
		}
		listeners = append(listeners, listener)
	}

	return listeners, nil
}
