package server

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type MultiChainRPC struct {
	*RPCServerCore
	beaconRPC *ChainRPC
	shardRPCs map[uint32]*ChainRPC
}

func NewMultiChainRPC(config *Config, logger *zap.Logger, beaconActor *ChainProvider,
	shardProviders map[uint32]*ChainProvider) *MultiChainRPC {
	rpc := &MultiChainRPC{
		RPCServerCore: NewRPCCore(config, logger),
		beaconRPC:     nil,
		shardRPCs:     map[uint32]*ChainRPC{},
	}

	rpc.beaconRPC = NewChainRPC(beaconActor, logger)
	for shardID, provider := range shardProviders {
		rpc.shardRPCs[shardID] = NewChainRPC(provider, logger)
	}

	return rpc
}

func (server *MultiChainRPC) Run(ctx context.Context) {
	rpcServeMux := http.NewServeMux()
	rpcServeMux.HandleFunc("/beacon/", server.HandleFunc(server.beaconRPC.CommandsMux))
	rpcServeMux.HandleFunc("/beacon/ws", server.WSHandleFunc())

	for shardID, chainRPC := range server.shardRPCs {
		path := fmt.Sprintf("/shard/%d", shardID)
		rpcServeMux.HandleFunc(path+"/", server.HandleFunc(chainRPC.CommandsMux))
		rpcServeMux.HandleFunc(path+"/ws", server.WSHandleFunc())
	}

	server.StartRPC(ctx, rpcServeMux)
}
