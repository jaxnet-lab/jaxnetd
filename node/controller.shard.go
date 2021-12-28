// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"errors"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/version"
)

func (chainCtl *chainController) EnableShard(shardID uint32) error {
	// chainCtl.wg.Add(1)
	// go chainCtl.runShardRoutine(shardID)
	return nil
}

func (chainCtl *chainController) DisableShard(shardID uint32) error {
	return nil
}

func (chainCtl *chainController) ShardCtl(id uint32) (*cprovider.ChainProvider, bool) {
	shardInfo, ok := chainCtl.shardsCtl[id]
	if !ok {
		return nil, ok
	}

	return shardInfo.ctl.chainProvider, ok
}

func (chainCtl *chainController) ListShards() jaxjson.ShardListResult {
	chainCtl.shardsMutex.RLock()
	defer chainCtl.shardsMutex.RUnlock()
	list := make(map[uint32]jaxjson.ShardInfo, len(chainCtl.shardsIndex.Shards))

	for _, shardInfo := range chainCtl.shardsIndex.Shards {
		list[shardInfo.ID] = jaxjson.ShardInfo{
			ID:                    shardInfo.ID,
			BeaconExpansionHeight: shardInfo.ExpansionHeight,
			BeaconExpansionHash:   shardInfo.ExpansionHash.String(),
			GenesisHash:           shardInfo.ShardGenesisHash.String(),
			Enabled:               shardInfo.Enabled,
			P2PPort:               shardInfo.P2PInfo.DefaultPort,
		}
	}

	return jaxjson.ShardListResult{
		Shards: list,
	}
}

func (chainCtl *chainController) GetNodeMetrics() jaxjson.GetNodeMetricsResult {
	statsMap := chainCtl.Stats()
	return jaxjson.GetNodeMetricsResult{Stats: statsMap, Version: version.GetVersion()}
}

func (chainCtl *chainController) GetChainMetrics() jaxjson.GetChainMetricsResult {
	beaconStats := chainCtl.beacon.Stats()
	var res jaxjson.GetChainMetricsResult
	res.NetName = chainCtl.cfg.Node.Net

	res.ChainStats = make(map[uint32]map[string]float64)
	res.ChainStats[0] = beaconStats

	for i, v := range chainCtl.shardsCtl {
		res.ChainStats[i] = v.ctl.Stats()
	}

	return res
}

// nolint: contextcheck
func (chainCtl *chainController) runShards() error {
	if err := chainCtl.syncShardsIndex(); err != nil {
		return err
	}

	shardsLimitOn := len(chainCtl.cfg.Node.Shards.EnabledShards) > 0
	enabledShards := make(map[uint32]struct{}, len(chainCtl.cfg.Node.Shards.EnabledShards))

	for _, shardID := range chainCtl.cfg.Node.Shards.EnabledShards {
		enabledShards[shardID] = struct{}{}
	}

	for _, info := range chainCtl.shardsIndex.Shards {
		if _, ok := enabledShards[info.ID]; shardsLimitOn && !ok {
			chainCtl.logger.Info().Uint32("shard", info.ShardInfo.ID).Msgf("shard disabled by configuration")
			continue
		}

		block, err := chainCtl.beacon.chainProvider.BlockChain().BlockByHash(&info.ExpansionHash)
		if err != nil {
			// todo: need to cope with the situation when the shard becomes an orphan
			return err
		}

		if !block.MsgBlock().Header.Version().ExpansionMade() {
			return errors.New("invalid start genesis block, expansion not made at this height")
		}

		err = info.P2PInfo.Update(chainCtl.cfg.Node.P2P.Listeners, info.ID, chainCtl.cfg.Node.P2P.ShardDefaultPort)
		if err != nil {
			return err
		}

		chainCtl.runShardRoutine(info.ID, info.P2PInfo, block)
	}

	return nil
}

func (chainCtl *chainController) shardsAutorunCallback(not *blockchain.Notification) {
	if not.Type != blockchain.NTBlockConnected {
		return
	}

	block, ok := not.Data.(*jaxutil.Block)
	if !ok {
		chainCtl.logger.Warn().Msg("block notification data is not a *jaxutil.Block")
		return
	}

	if !block.MsgBlock().Header.Version().ExpansionMade() {
		return
	}

	if err := chainCtl.saveNewShard(block); err != nil {
		return
	}

	opts := p2p.ListenOpts{}
	if err := opts.Update(chainCtl.cfg.Node.P2P.Listeners,
		block.MsgBlock().Header.BeaconHeader().Shards(), // todo: change this
		chainCtl.cfg.Node.P2P.ShardDefaultPort); err != nil {
		chainCtl.logger.Error().Err(err).Msg("unable to get free port")
	}

	chainCtl.shardsMutex.Lock()
	shardID := chainCtl.shardsIndex.AddShard(block, opts)
	chainCtl.shardsMutex.Unlock()

	chainCtl.runShardRoutine(shardID, opts, block)
}

func (chainCtl *chainController) runShardRoutine(shardID uint32, opts p2p.ListenOpts, block *jaxutil.Block) {
	chainCtl.ctlMutex.Lock()
	defer chainCtl.ctlMutex.Unlock()

	if interruptRequested(chainCtl.ctx) {
		chainCtl.logger.Error().
			Uint32("shard_id", shardID).
			Err(errors.New("can't create interrupt request")).
			Msg("shard run interrupted")

		return
	}

	shardChainCtx := chainctx.ShardChain(shardID, chainCtl.cfg.Node.ChainParams(), block.MsgBlock(), block.Height())
	chainCtl.shardsIndex.SetShardGenesis(shardID, shardChainCtx.Params().GenesisHash())

	nCtx, cancel := context.WithCancel(chainCtl.ctx)
	shardCtl := NewShardCtl(nCtx, chainCtl.logger, chainCtl.cfg, shardChainCtx, opts)

	// gbt worker state was initialized in cprovider.NewChainProvider
	if err := shardCtl.Init(chainCtl.beacon.chainProvider); err != nil {
		chainCtl.logger.Error().Err(err).Msg("Can't init shard chainCtl")
		cancel()
		return
	}

	chainCtl.shardsMutex.Lock()
	chainCtl.ports.Add(shardID, opts.DefaultPort)
	chainCtl.shardsCtl[shardID] = shardRO{
		ctl:    shardCtl,
		cancel: cancel,
		port:   opts.DefaultPort,
	}
	chainCtl.wg.Add(1)
	chainCtl.shardsMutex.Unlock()

	go func() {
		shardCtl.Run(nCtx)

		chainCtl.shardsMutex.Lock()
		chainCtl.wg.Done()
		delete(chainCtl.shardsCtl, shardID)
		chainCtl.shardsMutex.Unlock()
	}()

	shardRPC := rpc.NewShardRPC(shardCtl.ChainProvider(), shardCtl.p2pServer.P2PConnManager(), chainCtl.logger)
	chainCtl.rpc.server.AddShard(shardID, shardRPC)

	// todo: error for one shard
	//  invalid orange tree: coding bits size must be greater than zero: validation error
	// if chainCtl.cfg.Node.EnableCPUMiner && chainCtl.miner != nil {
	// 	chainCtl.miner.AddChain(shardID, cpuminer.Config{
	// 		ChainParams:            shardCtl.chainProvider.ChainParams,
	// 		BlockTemplateGenerator: shardCtl.chainProvider.BlkTmplGenerator(),
	// 		MiningAddrs:            shardCtl.chainProvider.MiningAddrs,
	// 		ProcessBlock:           shardCtl.chainProvider.SyncManager.ProcessBlock,
	// 		IsCurrent:              shardCtl.chainProvider.SyncManager.IsCurrent,
	// 		AutominingEnabled:      chainCtl.cfg.Node.AutominingEnabled,
	// 		AutominingThreshold:    chainCtl.cfg.Node.AutominingThreshold,
	// 	})
	// }
}

func (chainCtl *chainController) syncShardsIndex() error {
	chainCtl.shardsIndex = &Index{
		Shards: map[uint32]ShardInfo{},
	}
	chainCtl.beacon.shardsIndex = &Index{
		Shards: map[uint32]ShardInfo{},
	}

	err := chainCtl.beacon.chainProvider.DB.Update(func(tx database.Tx) error {
		if tx.Metadata().Bucket(chaindata.ShardCreationsBucketName) == nil {
			_, err := tx.Metadata().CreateBucket(chaindata.ShardCreationsBucketName)
			return err
		}

		idx := &Index{Shards: map[uint32]ShardInfo{}}
		shardsData, lastShardID := chaindata.DBGetShardGenesisInfo(tx)
		for shardID, data := range shardsData {
			idx.Shards[shardID] = ShardInfo{
				ShardInfo: chaindata.ShardInfo{
					ID:              data.ID,
					ExpansionHeight: data.ExpansionHeight,
					ExpansionHash:   data.ExpansionHash,
					SerialID:        data.SerialID,
				},
				Enabled: true,
			}
		}
		idx.LastShardID = lastShardID
		idx.LastBeaconHeight = idx.Shards[lastShardID].ExpansionHeight

		chainCtl.shardsIndex = idx
		chainCtl.beacon.shardsIndex = idx

		return nil
	})
	if err != nil {
		return err
	}

	var maxHeight int32
	snapshot := chainCtl.beacon.chainProvider.BlockChain().BestSnapshot()
	if snapshot != nil {
		maxHeight = snapshot.Height
	}
	if maxHeight == -1 {
		// nothing to index
		return nil
	}

	for height := chainCtl.shardsIndex.LastBeaconHeight; height < maxHeight; height++ {
		block, err := chainCtl.beacon.chainProvider.BlockChain().BlockByHeight(height)
		if err != nil {
			return err
		}

		msgBlock := block.MsgBlock()
		if !msgBlock.Header.Version().ExpansionMade() {
			continue
		}
		chainCtl.shardsIndex.AddShard(block, p2p.ListenOpts{})
		chainCtl.beacon.shardsIndex.AddShard(block, p2p.ListenOpts{})
	}

	return chainCtl.beacon.saveShardsIndex()
}

func (chainCtl *chainController) saveNewShard(block *jaxutil.Block) error {
	return chainCtl.beacon.chainProvider.DB.Update(func(tx database.Tx) error {
		serialID, _, err := chaindata.DBFetchBlockSerialID(tx, block.Hash())
		if err != nil {
			return err
		}

		return chaindata.DBStoreShardGenesisInfo(tx, block.MsgBlock().Header.BeaconHeader().Shards(), block.Height(), block.Hash(), serialID)
	})
}
