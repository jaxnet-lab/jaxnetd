// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chain/shard"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
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
			ID:            shardInfo.ID,
			LastVersion:   int32(shardInfo.LastVersion),
			GenesisHeight: shardInfo.GenesisHeight,
			GenesisHash:   shardInfo.GenesisHash,
			Enabled:       shardInfo.Enabled,
			P2PPort:       shardInfo.P2PInfo.DefaultPort,
		}
	}

	return jaxjson.ShardListResult{
		Shards: list,
	}
}

func (chainCtl *chainController) runShards() error {
	if err := chainCtl.syncShardsIndex(); err != nil {
		return err
	}

	for _, info := range chainCtl.shardsIndex.Shards {
		block, err := chainCtl.beacon.chainProvider.BlockChain().BlockByHeight(info.GenesisHeight)
		if err != nil {
			return err
		}

		version := block.MsgBlock().Header.Version()
		if !version.ExpansionMade() {
			return errors.New("invalid start genesis block, expansion not made at this height")
		}

		err = info.P2PInfo.Update(chainCtl.cfg.Node.P2P.Listeners, info.ID, chainCtl.cfg.Node.P2P.ShardDefaultPort)
		if err != nil {
			return err
		}

		chainCtl.runShardRoutine(info.ID, info.P2PInfo, block, true)
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

	version := block.MsgBlock().Header.Version()
	if !version.ExpansionMade() {
		return
	}

	opts := p2p.ListenOpts{}
	if err := opts.Update(chainCtl.cfg.Node.P2P.Listeners,
		block.MsgBlock().Header.BeaconHeader().Shards(), // todo: change this
		chainCtl.cfg.Node.P2P.ShardDefaultPort); err != nil {
		chainCtl.logger.Error().Err(err).Msg("unable to get free port")
	}

	shardID := chainCtl.shardsIndex.AddShard(block, opts)
	chainCtl.runShardRoutine(shardID, opts, block, true)
}

func (chainCtl *chainController) runShardRoutine(shardID uint32, opts p2p.ListenOpts, block *jaxutil.Block, autoInit bool) {
	if interruptRequested(chainCtl.ctx) {
		chainCtl.logger.Error().
			Uint32("shard_id", shardID).
			Err(errors.New("can't create interrupt request")).
			Msg("shard run interrupted")

		return
	}

	chainCtx := shard.Chain(shardID, chainCtl.cfg.Node.ChainParams(), block.MsgBlock())

	nCtx, cancel := context.WithCancel(chainCtl.ctx)
	shardCtl := NewShardCtl(nCtx, chainCtl.logger, chainCtl.cfg, chainCtx, opts)

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

	if autoInit {
		shardRPC := rpc.NewShardRPC(shardCtl.ChainProvider(), chainCtl.rpc.connMgr, chainCtl.logger)
		chainCtl.rpc.server.AddShard(shardID, shardRPC)

		if chainCtl.cfg.Node.EnableCPUMiner {
			chainCtl.logger.Warn().Msg("CPUMiner is not available.")
			// chainCtl.runShardMiner(shardCtl.ChainProvider())
		}
	}

	if chainCtl.cfg.Metrics.Enable {
		chainCtl.metrics.Add(MetricsOfChain(shardCtl, chainCtl.logger))
	}
}

func (chainCtl *chainController) saveShardsIndex() {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	content, err := json.Marshal(chainCtl.shardsIndex)
	if err != nil {
		chainCtl.logger.Error().Err(err).Msg("unable to marshal shards index")
		return
	}

	if err = ioutil.WriteFile(shardsFile, content, 0644); err != nil {
		chainCtl.logger.Error().Err(err).Msg("unable to write shards index")
		return
	}

	return
}

func (chainCtl *chainController) syncShardsIndex() error {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	chainCtl.shardsIndex = &Index{
		Shards: map[uint32]ShardInfo{},
	}

	if _, err := os.Stat(shardsFile); err == nil {
		file, err := ioutil.ReadFile(shardsFile)
		if err != nil {
			return err
		}

		err = json.Unmarshal(file, chainCtl.shardsIndex)
		if err != nil {
			return err
		}
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
		version := msgBlock.Header.Version()

		if !version.ExpansionMade() {
			continue
		}
		chainCtl.shardsIndex.AddShard(block, p2p.ListenOpts{})
	}

	chainCtl.saveShardsIndex()
	return nil
}
