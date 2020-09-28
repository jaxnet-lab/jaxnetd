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
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/shard"
	server2 "gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"go.uber.org/zap"
)

func (chainCtl *chainController) EnableShard(shardID uint32) error {
	chainCtl.wg.Add(1)

	// go chainCtl.runShardRoutine(shardID)

	return nil
}

func (chainCtl *chainController) ListShards() map[uint32]string {
	chainCtl.shardsMutex.RLock()
	list := map[uint32]string{}

	for id, ro := range chainCtl.shardsCtl {
		list[id] = ro.ctl.chain.Params().Name
	}

	return list
}

func (chainCtl *chainController) DisableShard(shardID uint32) error {
	return nil
}

func (chainCtl *chainController) NewShard(shardID uint32, height int32) error {
	chainCtl.wg.Add(1)

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

func (chainCtl *chainController) runShards() error {
	if err := chainCtl.syncShardsIndex(); err != nil {
		return err
	}

	defer chainCtl.writeShardsIndex()

	chainCtl.beacon.chainProvider.BlockChain.Subscribe(func(not *blockchain.Notification) {
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

	for shardID, info := range chainCtl.shardsIndex.Shards {
		err := chainCtl.NewShard(shardID, info.GenesisHeight)
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
		shardCtl.Run(chainCtl.ctx)

		chainCtl.shardsMutex.Lock()
		chainCtl.wg.Done()
		delete(chainCtl.shardsCtl, shardID)
		chainCtl.shardsMutex.Unlock()
	}()
}

func (chainCtl *chainController) writeShardsIndex() error {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	content, err := json.Marshal(chainCtl.shardsIndex)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(shardsFile, content, 0644)
}

func (chainCtl *chainController) syncShardsIndex() error {
	shardsFile := filepath.Join(chainCtl.cfg.DataDir, "shards.json")
	chainCtl.shardsIndex = &Index{
		Shards: map[uint32]ShardInfo{},
	}

	_, err := os.Stat(shardsFile)
	if os.IsNotExist(err) {
		if err = chainCtl.writeShardsIndex(); err != nil {
			return err
		}
	} else {
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

	return chainCtl.writeShardsIndex()
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
	p2pServer     *server2.P2PServer
	chainProvider *server2.ChainProvider
}

func NewShardCtl(ctx context.Context, log *zap.Logger, cfg *Config, chain chain.IChain) *ShardCtl {
	log = log.With(zap.String("chain", chain.Params().Name))

	return &ShardCtl{
		ctx:   ctx,
		cfg:   cfg,
		log:   log,
		chain: chain,
		dbCtl: DBCtl{},
	}
}

func (shardCtl ShardCtl) Init() error {

	// Load the block database.
	db, err := shardCtl.dbCtl.loadBlockDB(shardCtl.cfg.DataDir, shardCtl.chain, shardCtl.cfg.Node)
	if err != nil {
		shardCtl.log.Error("Can't load Block db", zap.Error(err))
		return err
	}

	defer func() {
		// Ensure the database is sync'd and closed on shutdown.
		shardCtl.log.Info("Gracefully shutting down the database...")
		if err := db.Close(); err != nil {
			shardCtl.log.Error("Can't close db", zap.Error(err))
		}
	}()

	shardCtl.chainProvider, err = server2.NewChainActor(shardCtl.ctx,
		shardCtl.cfg.Node.BeaconChain, shardCtl.chain, db, shardCtl.log)
	if err != nil {
		shardCtl.log.Error("unable to init ChainProvider for shard", zap.Error(err))
		return err
	}

	amgr := addrmgr.New(shardCtl.cfg.DataDir, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}
		return shardCtl.cfg.Node.P2P.Lookup(host)
	})

	shardCtl.log.Info("Run P2P Listener ", zap.Any("Listeners", shardCtl.cfg.Node.P2P.Listeners))

	// Create p2pServer and start it.
	shardCtl.p2pServer, err = server2.ShardServer(&shardCtl.cfg.Node.P2P, shardCtl.chainProvider, amgr)
	if err != nil {
		shardCtl.log.Error("Unable to start p2pServer",
			zap.Any("address", shardCtl.cfg.Node.P2P.Listeners), zap.Error(err))
		return err
	}
	return nil
}

func (shardCtl *ShardCtl) ChainProvider() *server2.ChainProvider {
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

	// go func() {
	// 	defer wg.Done()
	// 	shardCtl.rpcServer.Start(ctx)
	// }()

	<-ctx.Done()
	wg.Wait()
	shardCtl.log.Info("Chain p2p server shutdown complete")

	shardCtl.log.Info("Gracefully shutting down the database...")
	if err := shardCtl.chainProvider.DB.Close(); err != nil {
		shardCtl.log.Error("Can't close db", zap.Error(err))
	}
}
