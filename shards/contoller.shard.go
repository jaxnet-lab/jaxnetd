package shards

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"gitlab.com/jaxnet/core/shard.core.git/addrmgr"
	"gitlab.com/jaxnet/core/shard.core.git/database"
	"gitlab.com/jaxnet/core/shard.core.git/mining"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/shard"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	server2 "gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"go.uber.org/zap"
)

type shardRO struct {
	ctl    *ShardCtl
	cancel context.CancelFunc
}

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

	block, err := chainCtl.beacon.blockchain.BlockByHeight(height)
	if err != nil {
		return err
	}

	msgBlock := block.MsgBlock()
	version := msgBlock.Header.Version()

	if !version.ExpansionMade() {
		return errors.New("invalid start genesis block, expansion not made at this height")
	}
	// fixme

	chainCtl.runShardRoutine(shardID, msgBlock, height)

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

type ShardCtl struct {
	log   *zap.Logger
	chain chain.IChain
	cfg   *Config
	ctx   context.Context

	db        database.DB
	dbCtl     DBCtl
	p2pServer *server2.P2PServer
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

	cleanSmth, err := shardCtl.dbCtl.cleanIndexes(shardCtl.ctx, shardCtl.cfg, db)
	if cleanSmth || err != nil {
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
	shardCtl.p2pServer, err = server2.ShardServer(shardCtl.ctx, &shardCtl.cfg.Node.P2P, amgr, shardCtl.chain, db,
		shardCtl.log.With(zap.String("p2pServer", "Shard P2P")))
	if err != nil {
		shardCtl.log.Error("Unable to start p2pServer",
			zap.Any("address", shardCtl.cfg.Node.P2P.Listeners), zap.Error(err))
		return err
	}
	return nil
}

func (shardCtl *ShardCtl) ChainActor() (*server.ChainActor, error) {
	policy := mining.Policy{
		BlockMinWeight:    shardCtl.cfg.Node.P2P.BlockMinWeight,
		BlockMaxWeight:    shardCtl.cfg.Node.P2P.BlockMaxWeight,
		BlockMinSize:      shardCtl.cfg.Node.P2P.BlockMinSize,
		BlockMaxSize:      shardCtl.cfg.Node.P2P.BlockMaxSize,
		BlockPrioritySize: shardCtl.cfg.Node.P2P.BlockPrioritySize,
		TxMinFreeFee:      shardCtl.cfg.Node.P2P.MinRelayTxFeeValues,
	}
	blockTemplateGenerator := mining.NewBlkTmplGenerator(&policy,
		shardCtl.chain.Params(), shardCtl.p2pServer.TxMemPool, shardCtl.p2pServer.GetBlockChain(), shardCtl.p2pServer.TimeSource,
		shardCtl.p2pServer.SigCache, shardCtl.p2pServer.HashCache)

	_, err := shardCtl.cfg.Node.RPC.SetupRPCListeners()
	if err != nil {
		return nil, err
	}

	miningAddrs, err := shardCtl.cfg.Node.ParseMiningAddresses()
	if err != nil {
		return nil, err
	}

	return &server.ChainActor{
		StartupTime:  shardCtl.p2pServer.StartupTime,
		ConnMgr:      &server.RPCConnManager{Server: shardCtl.p2pServer},
		SyncMgr:      &server.RPCSyncMgr{Server: shardCtl.p2pServer, SyncMgr: shardCtl.p2pServer.SyncManager},
		TimeSource:   shardCtl.p2pServer.TimeSource,
		DB:           shardCtl.db,
		Generator:    blockTemplateGenerator,
		TxIndex:      shardCtl.p2pServer.TxIndex,
		AddrIndex:    shardCtl.p2pServer.AddrIndex,
		CfIndex:      shardCtl.p2pServer.CfIndex,
		FeeEstimator: shardCtl.p2pServer.FeeEstimator,
		MiningAddrs:  miningAddrs,

		// ShardsMgr:   shardCtl.shardsMgr,
		Chain:       shardCtl.p2pServer.GetBlockChain(),
		ChainParams: shardCtl.chain.Params(),
		TxMemPool:   shardCtl.p2pServer.TxMemPool,
	}, nil
}

func (shardCtl *ShardCtl) Run(ctx context.Context) {
	cleanIndexes, err := shardCtl.dbCtl.cleanIndexes(ctx, shardCtl.cfg, shardCtl.db)
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
	if err := shardCtl.db.Close(); err != nil {
		shardCtl.log.Error("Can't close db", zap.Error(err))
	}
}
