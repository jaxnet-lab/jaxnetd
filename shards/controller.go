package shards

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
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
	// -------------------------------

	beacon      BeaconCtl
	shardsCtl   map[uint32]shardRO
	shardsIndex *Index
	shardsMutex sync.RWMutex
}

func Controller(logger *zap.Logger) *chainController {
	res := &chainController{
		logger:    logger,
		shardsCtl: make(map[uint32]shardRO),
	}
	return res
}

func (chainCtl *chainController) Run(ctx context.Context, cfg *Config) error {
	chainCtl.cfg = cfg
	chainCtl.ctx, chainCtl.cancel = context.WithCancel(ctx)

	chainCtl.wg.Add(1)
	go func() {
		defer chainCtl.wg.Done()
		if err := chainCtl.runBeacon(chainCtl.ctx, cfg); err != nil {
			chainCtl.logger.Error("Beacon error", zap.Error(err))
		}
	}()

	if !cfg.Node.Shards.Enable {
		return nil
	}
	if err := chainCtl.syncShardsIndex(); err != nil {

		return err
	}
	defer chainCtl.updateShardsIndex()

	chainCtl.beacon.blockchain.Subscribe(func(not *blockchain.Notification) {
		if not.Type != blockchain.NTBlockConnected {
			return
		}
		block, ok := not.Data.(*btcutil.Block)
		if !ok {
			chainCtl.logger.Warn("block notification data is not a *btcutil.Block")
			return
		}

		msgBlock := block.MsgBlock()
		version := msgBlock.Header.Version()

		if !version.ExpansionMade() {
			return
		}

		chainCtl.shardsIndex.AddShard(block)
		chainCtl.runShardRoutine(chainCtl.shardsIndex.LastShardID, msgBlock, block.Height())

	})

	for shardID := range cfg.Node.Shards.IDs {
		chainCtl.wg.Add(1)
		var block *wire.MsgBlock
		var height int32
		// todo(mike)
		chainCtl.runShardRoutine(shardID, block, height)
	}

	// if len(chainCtl.shardsCtl) != len(cfg.Node.Shards.IDs) {
	// 	chainCtl.cancel()
	// 	chainCtl.wg.Wait()
	// 	return errors.New("some shardsCtl not started")
	// }

	<-ctx.Done()
	chainCtl.wg.Wait()
	return nil
}

func (chainCtl *chainController) updateShardsIndex() {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	file, err := os.OpenFile(shardsFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	err = json.NewEncoder(file).Encode(chainCtl.shardsIndex)
	if err != nil {
		return
	}
	return
}

func (chainCtl *chainController) syncShardsIndex() error {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	file, err := os.OpenFile(shardsFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	sIndex := &Index{
		Shards: map[uint32]ShardInfo{},
	}
	err = json.NewDecoder(file).Decode(sIndex)
	if err != nil {
		return err
	}

	var maxHeight int32
	snapshot := chainCtl.beacon.blockchain.BestSnapshot()
	if snapshot != nil {
		maxHeight = snapshot.Height
	}

	for height := sIndex.LastBeaconHeight; height < maxHeight; height++ {
		block, err := chainCtl.beacon.blockchain.BlockByHeight(height)
		if err != nil {
			return err
		}

		msgBlock := block.MsgBlock()
		version := msgBlock.Header.Version()

		if !version.ExpansionMade() {
			continue
		}
		sIndex.AddShard(block)
	}

	chainCtl.shardsIndex = sIndex
	err = json.NewEncoder(file).Encode(sIndex)
	if err != nil {
		return err
	}

	return nil
}

type ShardInfo struct {
	ID            uint32         `json:"id"`
	Name          string         `json:"name"`
	LastVersion   chain.BVersion `json:"last_version"`
	GenesisHeight int32          `json:"genesis_height"`
	GenesisHash   string         `json:"genesis_hash"`
	Enabled       bool           `json:"enabled"`
}

type Index struct {
	LastShardID      uint32               `json:"last_shard_id"`
	LastBeaconHeight int32                `json:"last_beacon_height"`
	Shards           map[uint32]ShardInfo `json:"shards"`
}

func (index *Index) AddShard(block *btcutil.Block) {
	index.LastShardID += 1

	if index.LastBeaconHeight < block.Height() {
		index.LastBeaconHeight = block.Height()
	}

	if index.Shards == nil {
		index.Shards = map[uint32]ShardInfo{}
	}

	index.Shards[index.LastShardID] = ShardInfo{
		ID:            index.LastShardID,
		Name:          "0",
		LastVersion:   block.MsgBlock().Header.Version(),
		GenesisHeight: block.Height(),
		GenesisHash:   block.Hash().String(),
		Enabled:       true,
	}
}
