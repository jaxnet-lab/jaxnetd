package shards

import (
	"context"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"go.uber.org/zap"
)

const (
	// blockDbNamePrefix is the prefix for the block database name.  The
	// database type is appended to this value to form the full block
	// database name.
	blockDbNamePrefix = "blocks"
)

type chainController struct {
	logger *zap.Logger
	shards map[uint32]chain.IChain
}

func Controller(logger *zap.Logger) *chainController {
	res := &chainController{
		logger: logger,
		shards: make(map[uint32]chain.IChain),
	}
	return res
}

func (c *chainController) Run(ctx context.Context, cfg *Config) error {
	go func() {
		if err := c.runBeacon(ctx, cfg); err != nil {
			c.logger.Error("Beacon error", zap.Error(err))
		}
	}()

	//go c.runShard(ctx, cfg, 1)
	//go c.runShard(ctx, cfg, 2)
	//c.runShard(ctx, cfg, 2)
	//c.runShard(ctx, cfg, 3)
	//
	return nil
}
