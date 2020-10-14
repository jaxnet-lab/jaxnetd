package node

import (
	"context"
	"errors"
	"sync"

	"gitlab.com/jaxnet/core/shard.core/network/netsync"
	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/network/rpc"
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

	beacon BeaconCtl

	// todo: repair
	miner *cpuminer.CPUMiner

	shardsCtl   map[uint32]shardRO
	shardsIndex *Index
	shardsMutex sync.RWMutex
	ports       *p2p.ChainsPortIndex
	rpc         rpcRO
	// -------------------------------

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

func (chainCtl *chainController) InitCPUMiner(connectedCount func() int32) *cpuminer.CPUMiner {
	chainCtl.miner = cpuminer.New(&cpuminer.Config{
		ChainParams:            chainCtl.beacon.chainProvider.ChainParams,
		BlockTemplateGenerator: chainCtl.beacon.BlkTmplGenerator(),
		MiningAddrs:            chainCtl.beacon.chainProvider.MiningAddrs,

		ProcessBlock:   chainCtl.beacon.chainProvider.SyncManager.ProcessBlock,
		IsCurrent:      chainCtl.beacon.chainProvider.SyncManager.IsCurrent,
		ConnectedCount: connectedCount,
	}, chainCtl.logger)

	return chainCtl.miner
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
	beaconRPC := rpc.NewBeaconRPC(chainCtl.beacon.ChainProvider(), connMgr,
		chainCtl.beacon.BlkTmplGenerator(), chainCtl.logger)

	shardRPCs := map[uint32]*rpc.ShardRPC{}
	for shardID, ro := range chainCtl.shardsCtl {
		shardRPCs[shardID] = rpc.NewShardRPC(ro.ctl.ChainProvider(), connMgr,
			ro.ctl.BlkTmplGenerator(beaconRPC.BlockGenerator), chainCtl.logger)
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
