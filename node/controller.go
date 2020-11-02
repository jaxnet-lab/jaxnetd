// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package node

import (
	"context"
	"errors"
	"sync"
	"time"

	"gitlab.com/jaxnet/core/shard.core/network/netsync"
	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/network/rpc"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/mining/cpuminer"
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
	ports       *p2p.ChainsPortIndex
	rpc         rpcRO
	// -------------------------------

	miner   *cpuminer.MultiMiner
	metrics IMetricManager
}

func Controller(logger *zap.Logger) *chainController {
	res := &chainController{
		logger:    logger,
		shardsCtl: make(map[uint32]shardRO),
		shardsIndex: &Index{
			LastShardID:      0,
			LastBeaconHeight: 0,
			Shards:           []ShardInfo{},
		},
		ports: p2p.NewPortsIndex(),
	}
	return res
}

func (chainCtl *chainController) Run(ctx context.Context, cfg *Config) error {
	// todo: fix after p2p refactoring
	cfg.Node.P2P.GetChainPort = chainCtl.ports.Get
	chainCtl.cfg = cfg
	chainCtl.ctx, chainCtl.cancel = context.WithCancel(ctx)

	if err := chainCtl.runBeacon(chainCtl.ctx, cfg); err != nil {
		chainCtl.logger.Error("Beacon error", zap.Error(err))
		return err
	}

	// todo: fix flag
	if cfg.Node.Shards.Enable {
		go func() {
			if err := chainCtl.runMetricsServer(chainCtl.ctx, cfg); err != nil {
				chainCtl.logger.Error("listen metrics server", zap.Error(err))
			}
		}()
	}

	if cfg.Node.Shards.Enable {
		if err := chainCtl.runShards(); err != nil {
			chainCtl.logger.Error("Shards error", zap.Error(err))
			return err
		}
		defer chainCtl.saveShardsIndex()
	}

	if cfg.Node.Shards.Autorun {
		chainCtl.beacon.chainProvider.BlockChain().Subscribe(chainCtl.shardsAutorunCallback)
	}

	if err := chainCtl.runRpc(chainCtl.ctx, cfg); err != nil {
		chainCtl.logger.Error("RPC ComposeHandlers error", zap.Error(err))
		return err
	}

	if chainCtl.cfg.Node.EnableCPUMiner {
		// chainCtl.InitCPUMiner(chainCtl.beacon.p2pServer.ConnectedCount)
		chainCtl.InitCPUMiner(func() int32 { return 1 })
		chainCtl.wg.Add(1)
		go func() {
			defer chainCtl.wg.Done()
			chainCtl.miner.Run(chainCtl.ctx)
		}()
	}

	<-ctx.Done()
	chainCtl.wg.Wait()
	return nil
}

func (chainCtl *chainController) InitCPUMiner(connectedCount func() int32) {
	minerSet := map[string]*cpuminer.Config{
		"beacon": {
			ChainParams:            chainCtl.beacon.chainProvider.ChainParams,
			BlockTemplateGenerator: chainCtl.beacon.chainProvider.BlkTmplGenerator(),
			MiningAddrs:            chainCtl.beacon.chainProvider.MiningAddrs,

			ProcessBlock:   chainCtl.beacon.chainProvider.SyncManager.ProcessBlock,
			IsCurrent:      chainCtl.beacon.chainProvider.SyncManager.IsCurrent,
			ConnectedCount: connectedCount,
		},
	}

	for _, ro := range chainCtl.shardsCtl {
		minerSet[ro.ctl.chain.Params().Name] = &cpuminer.Config{
			ChainParams:            ro.ctl.chainProvider.ChainParams,
			BlockTemplateGenerator: ro.ctl.chainProvider.BlkTmplGenerator(),
			MiningAddrs:            ro.ctl.chainProvider.MiningAddrs,
			ProcessBlock:           ro.ctl.chainProvider.SyncManager.ProcessBlock,
			IsCurrent:              ro.ctl.chainProvider.SyncManager.IsCurrent,
			ConnectedCount:         connectedCount,
		}
	}

	chainCtl.miner = cpuminer.NewMiner(minerSet, chainCtl.logger.With(zap.String("context", "miner")))
	return
}

func (chainCtl *chainController) runShardMiner(chainProvider *cprovider.ChainProvider) {
	chainCtl.miner.AddChainMiner(chainProvider.ChainCtx.Params().Name, &cpuminer.Config{
		ChainParams:            chainProvider.ChainParams,
		BlockTemplateGenerator: chainProvider.BlkTmplGenerator(),
		MiningAddrs:            chainProvider.MiningAddrs,
		ProcessBlock:           chainProvider.SyncManager.ProcessBlock,
		IsCurrent:              chainProvider.SyncManager.IsCurrent,
		ConnectedCount:         func() int32 { return 1 },
	})
}

func (chainCtl *chainController) runBeacon(ctx context.Context, cfg *Config) error {
	if interruptRequested(ctx) {
		return errors.New("can't create interrupt request")
	}

	chainCtl.beacon = NewBeaconCtl(ctx, chainCtl.logger, cfg)
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

type rpcRO struct {
	server  *rpc.MultiChainRPC
	beacon  *rpc.BeaconRPC
	node    *rpc.NodeRPC
	connMgr netsync.P2PConnManager
}

func (chainCtl *chainController) runRpc(ctx context.Context, cfg *Config) error {
	connMgr := chainCtl.beacon.p2pServer.P2PConnManager()

	nodeRPC := rpc.NewNodeRPC(chainCtl, chainCtl.logger)
	beaconRPC := rpc.NewBeaconRPC(chainCtl.beacon.ChainProvider(), connMgr, chainCtl.logger)

	shardRPCs := map[uint32]*rpc.ShardRPC{}
	for shardID, ro := range chainCtl.shardsCtl {
		shardRPCs[shardID] = rpc.NewShardRPC(ro.ctl.ChainProvider(), connMgr, chainCtl.logger)
	}

	chainCtl.rpc.server = rpc.NewMultiChainRPC(&cfg.Node.RPC, chainCtl.logger,
		nodeRPC, beaconRPC, shardRPCs)

	chainCtl.wg.Add(1)
	go func() {
		chainCtl.rpc.server.Run(ctx)
		chainCtl.wg.Done()
	}()

	chainCtl.rpc.node = nodeRPC
	chainCtl.rpc.beacon = beaconRPC
	chainCtl.rpc.connMgr = connMgr
	return nil
}

func (chainCtl *chainController) runMetricsServer(ctx context.Context, cfg *Config) error {
	childCtx, _ := context.WithCancel(ctx)
	chainCtl.logger.Info("Metrics Enabled")
	interval := cfg.Metrics.Interval
	if interval == 0 {
		interval = 5
	}
	port := cfg.Metrics.Port
	if port == 0 {
		port = 2112
	}

	chainCtl.metrics = Metrics(childCtx, time.Duration(interval)*time.Second)
	chainCtl.metrics.Add(
		ChainMetrics(chainCtl.beacon.chainProvider.BlockChain(), "beacon", chainCtl.logger),
		NodeMetrics(chainCtl.cfg, chainCtl.shardsIndex, chainCtl.logger),
	)

	return chainCtl.metrics.Listen("/metrics", port)
}
