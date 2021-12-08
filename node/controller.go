// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"errors"
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/node/mining/cpuminer"
)

const (
	// blockDBNamePrefix is the prefix for the block database name.  The
	// database type is appended to this value to form the full block
	// database name.
	blockDBNamePrefix = "blocks"
)

type chainController struct {
	logger   zerolog.Logger
	cfg      *Config
	ctlMutex sync.RWMutex
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

	miner *cpuminer.CPUMiner
}

// nolint: golint, revive
func Controller(logger zerolog.Logger) *chainController {
	res := &chainController{
		logger:    logger,
		shardsCtl: make(map[uint32]shardRO),
		shardsIndex: &Index{
			LastShardID:      0,
			LastBeaconHeight: 0,
			Shards:           map[uint32]ShardInfo{},
		},
		ports: p2p.NewPortsIndex(),
	}
	return res
}

// nolint
func (chainCtl *chainController) Run(ctx context.Context, cfg *Config) error {
	// todo: fix after p2p refactoring
	cfg.Node.P2P.GetChainPort = chainCtl.ports.Get
	chainCtl.cfg = cfg
	chainCtl.ctx, chainCtl.cancel = context.WithCancel(ctx)

	if err := chainCtl.runBeacon(chainCtl.ctx, cfg); err != nil {
		chainCtl.logger.Error().Err(err).Msg("Beacon error")
		return err
	}

	if err := chainCtl.runRPC(chainCtl.ctx, cfg); err != nil {
		chainCtl.logger.Error().Err(err).Msg("RPC ComposeHandlers error")
		return err
	}

	if cfg.Node.Shards.Enable {
		if err := chainCtl.runShards(); err != nil {
			chainCtl.logger.Error().Err(err).Msg("Shards error")
			return err
		}
		defer chainCtl.saveShardsIndex()
	}

	if cfg.Node.Shards.Autorun {
		chainCtl.beacon.chainProvider.BlockChain().Subscribe(chainCtl.shardsAutorunCallback)
	}

	if chainCtl.cfg.Node.EnableCPUMiner {
		if len(chainCtl.beacon.chainProvider.MiningAddrs) == 0 {
			err := errors.New("you need so specify mining addresses in config in order to run CPU miner")
			return err
		}
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
	miningAddrs := chainCtl.beacon.chainProvider.MiningAddrs
	beacon := cpuminer.Config{
		ChainParams:            chainCtl.beacon.chainProvider.ChainParams,
		BlockTemplateGenerator: chainCtl.beacon.chainProvider.BlkTmplGenerator(),

		ProcessBlock:   chainCtl.beacon.chainProvider.SyncManager.ProcessBlock,
		IsCurrent:      chainCtl.beacon.chainProvider.SyncManager.IsCurrent,
		ConnectedCount: connectedCount,
	}

	shards := map[uint32]cpuminer.Config{}
	for shardID, ro := range chainCtl.shardsCtl {
		shards[shardID] = cpuminer.Config{
			ChainParams:            ro.ctl.chainProvider.ChainParams,
			BlockTemplateGenerator: ro.ctl.chainProvider.BlkTmplGenerator(),
			MiningAddrs:            ro.ctl.chainProvider.MiningAddrs,
			ProcessBlock:           ro.ctl.chainProvider.SyncManager.ProcessBlock,
			IsCurrent:              ro.ctl.chainProvider.SyncManager.IsCurrent,
			ConnectedCount:         connectedCount,
		}
	}

	chainCtl.miner = cpuminer.New(beacon, shards, miningAddrs[0],
		chainCtl.logger.With().Str("ctx", "miner").Logger())
}

func (chainCtl *chainController) runBeacon(ctx context.Context, cfg *Config) error {
	if interruptRequested(ctx) {
		return errors.New("can't create interrupt request")
	}

	chainCtl.beacon = NewBeaconCtl(ctx, chainCtl.logger, cfg)
	if err := chainCtl.beacon.Init(); err != nil {
		chainCtl.logger.Error().Err(err).Msg("Can't init Beacon chainCtl")
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

func (chainCtl *chainController) runRPC(ctx context.Context, cfg *Config) error {
	connMgr := chainCtl.beacon.p2pServer.P2PConnManager()

	chainCtl.logger.Info().Msg("Create WS RPC server")

	nodeRPC := rpc.NewNodeRPC(chainCtl.beacon.ChainProvider(), chainCtl, chainCtl.logger, chainCtl)
	beaconRPC := rpc.NewBeaconRPC(chainCtl.beacon.ChainProvider(), connMgr, chainCtl.logger)

	shardRPCs := map[uint32]*rpc.ShardRPC{}
	for shardID, ro := range chainCtl.shardsCtl {
		shardRPCs[shardID] = rpc.NewShardRPC(ro.ctl.ChainProvider(), ro.ctl.p2pServer.P2PConnManager(), chainCtl.logger)
	}

	chainCtl.rpc.server = rpc.NewMultiChainRPC(&cfg.Node.RPC, chainCtl.logger,
		nodeRPC, beaconRPC, shardRPCs)

	chainCtl.wg.Add(1)
	go func() {
		chainCtl.logger.Info().Msg("Run RPC server")
		chainCtl.rpc.server.Run(ctx)
		chainCtl.wg.Done()
	}()

	chainCtl.rpc.node = nodeRPC
	chainCtl.rpc.beacon = beaconRPC
	chainCtl.rpc.connMgr = connMgr
	return nil
}

func (chainCtl *chainController) Stats() map[string]float64 {
	var activeClients int32
	if chainCtl.rpc.server != nil {
		activeClients = chainCtl.rpc.server.ActiveClients()
	}

	var lastShardID uint32
	var activeShards int
	if chainCtl.shardsIndex != nil {
		lastShardID = chainCtl.shardsIndex.LastShardID
		activeShards = len(chainCtl.shardsIndex.Shards)
	}

	return map[string]float64{
		"shards_count":        float64(lastShardID),
		"active_shards_count": float64(activeShards),
		"rpc_active_clients":  float64(activeClients),
	}
}
