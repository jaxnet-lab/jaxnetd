package shards

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
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

func (chainCtl *chainController) updateShardsIndex() error {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	file, err := os.OpenFile(shardsFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	err = json.NewEncoder(file).Encode(chainCtl.shardsIndex)
	if err != nil {
		return err
	}
	return nil
}

func (chainCtl *chainController) syncShardsIndex() error {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	_, err := os.Stat(shardsFile)
	if os.IsNotExist(err) {
		if err = chainCtl.updateShardsIndex(); err != nil {
			return err
		}
	}

	file, err := ioutil.ReadFile(shardsFile)
	if err != nil {
		return err
	}
	sIndex := &Index{
		Shards: map[uint32]ShardInfo{},
	}
	err = json.Unmarshal(file, sIndex)
	if err != nil {
		return err
	}

	var maxHeight int32
	snapshot := chainCtl.beacon.blockchain.BestSnapshot()
	if snapshot != nil {
		maxHeight = snapshot.Height
	}
	if maxHeight == -1 {
		// nothing to index
		return nil
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

	content, err := json.Marshal(sIndex)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(shardsFile, content, 0644)
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
