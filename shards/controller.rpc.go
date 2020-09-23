package shards

import (
	"context"

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
