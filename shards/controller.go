package shards

import (
	"context"
	"sync"

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
	cfg    *Config

	// controller runtime
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	beacon      chain.IChain
	shards      map[uint32]chain.IChain
	shardsMutex sync.RWMutex

	// -------------------------------
}

func Controller(logger *zap.Logger) *chainController {
	res := &chainController{
		logger: logger,
		shards: make(map[uint32]chain.IChain),
	}
	return res
}

func (ctrl *chainController) Run(ctx context.Context, cfg *Config) error {
	ctrl.cfg = cfg
	ctrl.ctx, ctrl.cancel = context.WithCancel(ctx)

	ctrl.wg.Add(1)
	go func() {
		defer ctrl.wg.Done()
		if err := ctrl.runBeacon(ctrl.ctx, cfg); err != nil {
			ctrl.logger.Error("Beacon error", zap.Error(err))
		}
	}()

	if !cfg.Node.Shards.Enable {
		return nil
	}

	for shardID := range cfg.Node.Shards.IDs {
		ctrl.wg.Add(1)
		go ctrl.runShardRoutine(shardID)
	}

	// if len(ctrl.shards) != len(cfg.Node.Shards.IDs) {
	// 	ctrl.cancel()
	// 	ctrl.wg.Wait()
	// 	return errors.New("some shards not started")
	// }

	<-ctx.Done()
	ctrl.wg.Wait()
	return nil
}
