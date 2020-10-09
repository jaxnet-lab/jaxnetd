package node

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/node/chain/shard"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"go.uber.org/zap"
)

func (chainCtl *chainController) EnableShard(shardID uint32) error {
	// chainCtl.wg.Add(1)
	// go chainCtl.runShardRoutine(shardID)
	return nil
}

func (chainCtl *chainController) DisableShard(shardID uint32) error {
	return nil
}

func (chainCtl *chainController) ListShards() []btcjson.ShardInfo {
	chainCtl.shardsMutex.RLock()
	defer chainCtl.shardsMutex.RUnlock()
	list := make([]btcjson.ShardInfo, 0, len(chainCtl.shardsIndex.Shards))

	for id, shardInfo := range chainCtl.shardsIndex.Shards {
		list[id] = btcjson.ShardInfo{
			ID:            shardInfo.ID,
			LastVersion:   int32(shardInfo.LastVersion),
			GenesisHeight: shardInfo.GenesisHeight,
			GenesisHash:   shardInfo.GenesisHash,
			Enabled:       shardInfo.Enabled,
		}
	}

	return list
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

		err = info.P2PInfo.Update(chainCtl.cfg.Node.P2P.Listeners)
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

	block, ok := not.Data.(*btcutil.Block)
	if !ok {
		chainCtl.logger.Warn("block notification data is not a *btcutil.Block")
		return
	}

	version := block.MsgBlock().Header.Version()
	if !version.ExpansionMade() {
		return
	}

	port, err := p2p.GetFreePort()
	if err != nil {
		chainCtl.logger.Error("unable to get free port",
			zap.Error(err))
	}

	opts := p2p.ListenOpts{
		DefaultPort: strconv.Itoa(port),
		Listeners:   p2p.SetPortForListeners(chainCtl.cfg.Node.P2P.Listeners, port),
	}
	shardID := chainCtl.shardsIndex.AddShard(block, opts)

	chainCtl.runShardRoutine(shardID, opts, block)
}

func (chainCtl *chainController) runShardRoutine(shardID uint32, opts p2p.ListenOpts, block *btcutil.Block) {
	if interruptRequested(chainCtl.ctx) {
		chainCtl.logger.Error("shard run interrupted",
			zap.Uint32("shard_id", shardID),
			zap.Error(errors.New("can't create interrupt request")))
		return
	}

	chainCtx := shard.Chain(shardID, chainCtl.cfg.Node.ChainParams(),
		block.MsgBlock().Header.BeaconHeader())

	nCtx, cancel := context.WithCancel(chainCtl.ctx)
	shardCtl := NewShardCtl(nCtx, chainCtl.logger, chainCtl.cfg, chainCtx, opts)
	if err := shardCtl.Init(); err != nil {
		chainCtl.logger.Error("Can't init shard chainCtl", zap.Error(err))
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
}

func (chainCtl *chainController) saveShardsIndex() {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	content, err := json.Marshal(chainCtl.shardsIndex)
	if err != nil {
		chainCtl.logger.Error("unable to marshal shards index", zap.Error(err))
		return
	}

	if err = ioutil.WriteFile(shardsFile, content, 0644); err != nil {
		chainCtl.logger.Error("unable to write shards index", zap.Error(err))
		return
	}

	return
}

func (chainCtl *chainController) syncShardsIndex() error {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	chainCtl.shardsIndex = &Index{
		Shards: []ShardInfo{},
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
