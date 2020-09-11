package shards

import (
	"context"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
)

func (c *chainController) runRpc(ctx context.Context, cfg *Config) error {
	srv, err := server.RpcServer(&cfg.Node.Rpc, c.logger)
	if err != nil{
		return err
	}
	go srv.Start()
	return nil
}
