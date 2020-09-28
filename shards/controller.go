package shards

import (
	"context"
	"errors"
	"sync"

	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
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
	// -------------------------------

	// controller runtime
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	beacon      BeaconCtl
	shardsCtl   map[uint32]shardRO
	shardsIndex *Index
	shardsMutex sync.RWMutex
	// -------------------------------

}

func Controller(logger *zap.Logger) *chainController {
	res := &chainController{
		logger:    logger,
		shardsCtl: make(map[uint32]shardRO),
		shardsIndex: &Index{
			LastShardID:      0,
			LastBeaconHeight: 0,
			Shards:           map[uint32]ShardInfo{},
		},
	}
	return res
}

func (chainCtl *chainController) Run(ctx context.Context, cfg *Config) error {
	chainCtl.cfg = cfg
	chainCtl.ctx, chainCtl.cancel = context.WithCancel(ctx)

	if err := chainCtl.runBeacon(chainCtl.ctx, cfg); err != nil {
		chainCtl.logger.Error("Beacon error", zap.Error(err))
		return err
	}

	if cfg.Node.Shards.Enable {
		if err := chainCtl.runShards(); err != nil {
			chainCtl.logger.Error("Shards error", zap.Error(err))
			return err
		}
	}

	if err := chainCtl.runRpc(ctx, cfg); err != nil {
		chainCtl.logger.Error("RPC Init error", zap.Error(err))
		return err
	}

	<-ctx.Done()
	chainCtl.wg.Wait()
	return nil
}

func (chainCtl *chainController) runBeacon(ctx context.Context, cfg *Config) error {
	if interruptRequested(ctx) {
		return errors.New("can't create interrupt request")
	}

	chainCtl.beacon = NewBeaconCtl(ctx, chainCtl.logger, cfg, chainCtl)
	if err := chainCtl.beacon.Init(); err != nil {
		chainCtl.logger.Error("Can't init Beacon chainCtl", zap.Error(err))
		return err
	}

	chainCtl.wg.Add(1)
	go func() {
		chainCtl.beacon.Run(ctx)
		chainCtl.wg.Done()
	}()

	return nil
}

func (chainCtl *chainController) runRpc(ctx context.Context, cfg *Config) error {
	beaconActor := chainCtl.beacon.ChainProvider()

	var shardsProviders = map[uint32]*server.ChainProvider{}
	for id, ro := range chainCtl.shardsCtl {
		shardsProviders[id] = ro.ctl.ChainProvider()
	}

	srv := server.NewMultiChainRPC(&cfg.Node.RPC, chainCtl.logger, beaconActor, shardsProviders)
	chainCtl.wg.Add(1)

	go func() {
		srv.Run(ctx)
		chainCtl.wg.Done()

	}()

	return nil
}
