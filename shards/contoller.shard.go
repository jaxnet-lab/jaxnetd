package shards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gitlab.com/jaxnet/core/shard.core.git/addrmgr"
	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/shard"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
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

func (chainCtl *chainController) NewShard(shardID uint32, height int32) error {
	block, err := chainCtl.beacon.chainProvider.BlockChain.BlockByHeight(height)
	if err != nil {
		return err
	}

	msgBlock := block.MsgBlock()
	version := msgBlock.Header.Version()

	if !version.ExpansionMade() {
		return errors.New("invalid start genesis block, expansion not made at this height")
	}

	chainCtl.shardsIndex.AddShard(block)
	chainCtl.runShardRoutine(shardID, msgBlock, height)

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

	msgBlock := block.MsgBlock()
	version := msgBlock.Header.Version()

	if !version.ExpansionMade() {
		return
	}

	chainCtl.shardsIndex.AddShard(block)
	chainCtl.runShardRoutine(chainCtl.shardsIndex.LastShardID, msgBlock, block.Height())
}

func (chainCtl *chainController) runShards() error {
	if err := chainCtl.syncShardsIndex(); err != nil {
		return err
	}

	for _, info := range chainCtl.shardsIndex.Shards {
		err := chainCtl.NewShard(info.ID, info.GenesisHeight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (chainCtl *chainController) runShardRoutine(shardID uint32, block *wire.MsgBlock, height int32) {
	if interruptRequested(chainCtl.ctx) {
		chainCtl.logger.Error("shard run interrupted",
			zap.Uint32("shard_id", shardID),
			zap.Error(errors.New("can't create interrupt request")))
		return
	}

	nCtx, cancel := context.WithCancel(chainCtl.ctx)
	iChain := shard.Chain(shardID, chainCtl.cfg.Node.ChainParams(), block, height)

	shardCtl := NewShardCtl(nCtx, chainCtl.logger, chainCtl.cfg, iChain)
	if err := shardCtl.Init(); err != nil {
		chainCtl.logger.Error("Can't init shard chainCtl", zap.Error(err))
		return
	}

	chainCtl.shardsMutex.Lock()
	chainCtl.shardsCtl[shardID] = shardRO{
		ctl:    shardCtl,
		cancel: cancel,
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
	snapshot := chainCtl.beacon.chainProvider.BlockChain.BestSnapshot()
	if snapshot != nil {
		maxHeight = snapshot.Height
	}
	if maxHeight == -1 {
		// nothing to index
		return nil
	}

	for height := chainCtl.shardsIndex.LastBeaconHeight; height < maxHeight; height++ {
		block, err := chainCtl.beacon.chainProvider.BlockChain.BlockByHeight(height)
		if err != nil {
			return err
		}

		msgBlock := block.MsgBlock()
		version := msgBlock.Header.Version()

		if !version.ExpansionMade() {
			continue
		}
		chainCtl.shardsIndex.AddShard(block)
	}

	chainCtl.saveShardsIndex()
	return nil
}

type ShardInfo struct {
	ID            uint32         `json:"id"`
	LastVersion   chain.BVersion `json:"last_version"`
	GenesisHeight int32          `json:"genesis_height"`
	GenesisHash   string         `json:"genesis_hash"`
	Enabled       bool           `json:"enabled"`
}

type Index struct {
	LastShardID      uint32      `json:"last_shard_id"`
	LastBeaconHeight int32       `json:"last_beacon_height"`
	Shards           []ShardInfo `json:"shards"`
}

func (index *Index) AddShard(block *btcutil.Block) {
	index.LastShardID += 1

	if index.LastBeaconHeight < block.Height() {
		index.LastBeaconHeight = block.Height()
	}

	if index.Shards == nil {
		index.Shards = []ShardInfo{}
	}

	index.Shards = append(index.Shards, ShardInfo{
		ID:            index.LastShardID,
		LastVersion:   block.MsgBlock().Header.Version(),
		GenesisHeight: block.Height(),
		GenesisHash:   block.Hash().String(),
		Enabled:       true,
	})
}

type shardRO struct {
	ctl    *ShardCtl
	cancel context.CancelFunc
}

type ShardCtl struct {
	log   *zap.Logger
	chain chain.IChain
	cfg   *Config
	ctx   context.Context

	dbCtl         DBCtl
	p2pServer     *server.P2PServer
	chainProvider *server.ChainProvider
}

func NewShardCtl(ctx context.Context, log *zap.Logger, cfg *Config, chain chain.IChain) *ShardCtl {
	log = log.With(zap.String("chain", chain.Params().Name))

	return &ShardCtl{
		ctx:   ctx,
		cfg:   cfg,
		log:   log,
		chain: chain,
		dbCtl: DBCtl{logger: log},
	}
}

func (shardCtl *ShardCtl) Init() error {
	// Load the block database.
	db, err := shardCtl.dbCtl.loadBlockDB(shardCtl.cfg.DataDir, shardCtl.chain, shardCtl.cfg.Node)
	if err != nil {
		shardCtl.log.Error("Can't load Block db", zap.Error(err))
		return err
	}

	shardCtl.chainProvider, err = server.NewChainActor(shardCtl.ctx,
		shardCtl.cfg.Node.BeaconChain, shardCtl.chain, db, shardCtl.log)
	if err != nil {
		shardCtl.log.Error("unable to init ChainProvider for shard", zap.Error(err))
		return err
	}

	addrManager := addrmgr.New(shardCtl.cfg.DataDir, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}
		return shardCtl.cfg.Node.P2P.Lookup(host)
	})

	shardCtl.log.Info("Run P2P Listener ", zap.Any("Listeners", shardCtl.cfg.Node.P2P.Listeners))

	// Create p2pServer and start it.
	shardCtl.p2pServer, err = server.ShardServer(&shardCtl.cfg.Node.P2P, shardCtl.chainProvider, addrManager)
	if err != nil {
		shardCtl.log.Error("Unable to start p2pServer",
			zap.Any("address", shardCtl.cfg.Node.P2P.Listeners), zap.Error(err))
		return err
	}

	return shardCtl.chainProvider.SetP2PProvider(shardCtl.p2pServer)
}

func (shardCtl *ShardCtl) ChainProvider() *server.ChainProvider {
	return shardCtl.chainProvider
}

func (shardCtl *ShardCtl) Run(ctx context.Context) {
	cleanIndexes, err := shardCtl.dbCtl.cleanIndexes(ctx, shardCtl.cfg, shardCtl.chainProvider.DB)
	if cleanIndexes {
		shardCtl.log.Info("clean db indexes")
		return
	}

	if err != nil {
		shardCtl.log.Error("failed to clean indexes", zap.Error(err))
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		shardCtl.p2pServer.Run(ctx)
	}()

	<-ctx.Done()
	wg.Wait()
	shardCtl.log.Info("Chain p2p server shutdown complete")

	shardCtl.log.Info("Gracefully shutting down the database...")
	if err := shardCtl.chainProvider.DB.Close(); err != nil {
		shardCtl.log.Error("Can't close db", zap.Error(err))
	}
}
