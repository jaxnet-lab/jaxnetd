package node

import (
	"context"
	"fmt"
	"net"
	"strings"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/node/chain"
	"gitlab.com/jaxnet/core/shard.core/node/chain/shard"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/mining"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
	"go.uber.org/zap"
)

type ShardInfo struct {
	ID            uint32         `json:"id"`
	LastVersion   wire.BVersion  `json:"last_version"`
	GenesisHeight int32          `json:"genesis_height"`
	GenesisHash   string         `json:"genesis_hash"`
	Enabled       bool           `json:"enabled"`
	P2PInfo       p2p.ListenOpts `json:"p2p_info"`
}

type Index struct {
	LastShardID      uint32      `json:"last_shard_id"`
	LastBeaconHeight int32       `json:"last_beacon_height"`
	Shards           []ShardInfo `json:"shards"`
}

func (index *Index) AddShard(block *btcutil.Block, opts p2p.ListenOpts) uint32 {
	index.LastShardID += 1

	if index.LastBeaconHeight < block.Height() {
		index.LastBeaconHeight = block.Height()
	}

	index.Shards = append(index.Shards, ShardInfo{
		ID:            index.LastShardID,
		LastVersion:   block.MsgBlock().Header.Version(),
		GenesisHeight: block.Height(),
		GenesisHash:   block.Hash().String(),
		Enabled:       true,
		P2PInfo:       opts,
	})
	return index.LastShardID
}

type shardRO struct {
	ctl    *ShardCtl
	port   string
	cancel context.CancelFunc
}

type ShardCtl struct {
	log   *zap.Logger
	chain chain.IChainCtx
	cfg   *Config
	ctx   context.Context

	dbCtl         DBCtl
	p2pServer     *p2p.Server
	chainProvider *cprovider.ChainProvider
	listenCfg     p2p.ListenOpts
}

func NewShardCtl(ctx context.Context, log *zap.Logger, cfg *Config, chain chain.IChainCtx, listenCfg p2p.ListenOpts) *ShardCtl {
	log = log.With(zap.String("chain", chain.Params().Name))

	return &ShardCtl{
		ctx:       ctx,
		cfg:       cfg,
		log:       log,
		chain:     chain,
		dbCtl:     DBCtl{logger: log},
		listenCfg: listenCfg,
	}
}

func (shardCtl *ShardCtl) Init() error {
	// Load the block database.
	db, err := shardCtl.dbCtl.loadBlockDB(shardCtl.cfg.DataDir, shardCtl.chain, shardCtl.cfg.Node)
	if err != nil {
		shardCtl.log.Error("Can't load Block db", zap.Error(err))
		return err
	}

	shardCtl.chainProvider, err = cprovider.NewChainProvider(shardCtl.ctx,
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
	shardCtl.p2pServer, err = p2p.NewServer(&shardCtl.cfg.Node.P2P, shardCtl.chainProvider,
		addrManager, shardCtl.listenCfg)
	if err != nil {
		shardCtl.log.Error("Unable to start p2pServer",
			zap.Any("address", shardCtl.cfg.Node.P2P.Listeners), zap.Error(err))
		return err
	}

	return shardCtl.chainProvider.SetP2PProvider(shardCtl.p2pServer)
}

func (shardCtl *ShardCtl) ChainProvider() *cprovider.ChainProvider {
	return shardCtl.chainProvider
}

func (shardCtl *ShardCtl) BlkTmplGenerator(
	beaconBlockGen func(useCoinbase bool) (mining.BlockTemplate, error)) *mining.BlkTmplGenerator {
	// Create the mining policy and block template generator based on the
	// configuration options.
	policy := mining.Policy{
		BlockMinWeight:    shardCtl.cfg.Node.BeaconChain.BlockMinWeight,
		BlockMaxWeight:    shardCtl.cfg.Node.BeaconChain.BlockMaxWeight,
		BlockMinSize:      shardCtl.cfg.Node.BeaconChain.BlockMinSize,
		BlockMaxSize:      shardCtl.cfg.Node.BeaconChain.BlockMaxSize,
		BlockPrioritySize: shardCtl.cfg.Node.BeaconChain.BlockPrioritySize,
		TxMinFreeFee:      shardCtl.cfg.Node.BeaconChain.MinRelayTxFeeValues,
	}
	return mining.NewBlkTmplGenerator(&policy,
		shardCtl.chainProvider.ChainCtx,
		&shard.HeaderGenerator{BlockGenerator: beaconBlockGen},
		shardCtl.chainProvider.TxMemPool,
		shardCtl.chainProvider.BlockChain())
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

	shardCtl.p2pServer.Run(ctx)

	<-ctx.Done()

	shardCtl.log.Info("Chain p2p server shutdown complete")

	shardCtl.log.Info("Gracefully shutting down the database...")
	if err := shardCtl.chainProvider.DB.Close(); err != nil {
		shardCtl.log.Error("Can't close db", zap.Error(err))
	}
}
