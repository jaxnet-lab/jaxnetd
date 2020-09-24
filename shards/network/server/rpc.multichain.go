package server

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type MultiChainRPC struct {
	*RPCServerCore
	beaconActor *ChainRPC
	shardActors map[uint32]*ChainRPC
}

func NewMultiChainRPC(config *Config, logger *zap.Logger, beaconActor *ChainActor,
	shardActors map[uint32]*ChainActor) *MultiChainRPC {
	rpc := &MultiChainRPC{
		RPCServerCore: NewRPCCore(config, logger),
		beaconActor:   nil,
		shardActors:   map[uint32]*ChainRPC{},
	}

	rpc.beaconActor = NewChainRPC(beaconActor, logger)
	for shardID, actor := range shardActors {
		rpc.shardActors[shardID] = NewChainRPC(actor, logger)
	}

	return rpc
}

func (server *MultiChainRPC) Run(ctx context.Context) {
	// todo(mike)
	rpcServeMux := http.NewServeMux()

	for shardID, chainRPC := range server.shardActors {
		path := fmt.Sprintf("/shard/%d", shardID)
		rpcServeMux.HandleFunc(path+"/", server.HandleFunc(chainRPC.CommandsMux))
		// Websocket endpoint.
		rpcServeMux.HandleFunc(path+"/ws", server.WSHandleFunc())
	}

	server.StartRPC(ctx, rpcServeMux)
}
