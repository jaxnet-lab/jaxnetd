package shards

import (
	"context"
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
}

func Controller(logger *zap.Logger) *chainController {
	res := &chainController{
		logger: logger,
	}
	return res
}

func (c *chainController) runShard() error {
	return nil
}

func (c *chainController) Run(ctx context.Context, cfg *Config) error {
	go func() {
		if err := c.runBeacon(cfg); err != nil {
			c.logger.Error("Beacon error", zap.Error(err))
		}
	}()

	c.runRpc(ctx, cfg)

	return nil
}
