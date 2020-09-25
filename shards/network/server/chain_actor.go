package server

import (
	"context"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/blockchain/indexers"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/database"
	"gitlab.com/jaxnet/core/shard.core.git/mempool"
	"gitlab.com/jaxnet/core/shard.core.git/mining"
	"gitlab.com/jaxnet/core/shard.core.git/mining/cpuminer"
	"gitlab.com/jaxnet/core/shard.core.git/netsync"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
	"go.uber.org/zap"
)

type ChainRuntimeConfig struct {
	SigCacheMaxSize     uint     `yaml:"sig_cache_max_size" long:"sigcachemaxsize" description:"The maximum number of entries in the signature verification cache"`
	AddCheckpoints      []string `yaml:"add_checkpoints" long:"addcheckpoint" description:"Add a custom checkpoint.  Format: '<height>:<hash>'"`
	AddrIndex           bool     `yaml:"addr_index" long:"addrindex" description:"Maintain a full address-based transaction index which makes the searchrawtransactions RPC available"`
	MaxPeers            int      `yaml:"max_peers" long:"maxpeers" description:"Max number of inbound and outbound peers"`
	BlockMaxSize        uint32   `yaml:"block_max_size" long:"blockmaxsize" description:"Maximum block size in bytes to be used when creating a block"`
	BlockMinSize        uint32   `yaml:"block_min_size" long:"blockminsize" description:"Mininum block size in bytes to be used when creating a block"`
	BlockMaxWeight      uint32   `yaml:"block_max_weight" long:"blockmaxweight" description:"Maximum block weight to be used when creating a block"`
	BlockMinWeight      uint32   `yaml:"block_min_weight" long:"blockminweight" description:"Mininum block weight to be used when creating a block"`
	BlockPrioritySize   uint32   `yaml:"block_priority_size" long:"blockprioritysize" description:"Size in bytes for high-priority/low-fee transactions when creating a block"`
	TxIndex             bool     `yaml:"tx_index" long:"txindex" description:"Maintain a full hash-based transaction index which makes all transactions available via the getrawtransaction RPC"`
	NoRelayPriority     bool     `yaml:"no_relay_priority" long:"norelaypriority" description:"Do not require free or low-fee transactions to have high priority for relaying"`
	RejectReplacement   bool     `yaml:"reject_replacement" long:"rejectreplacement" description:"Reject transactions that attempt to replace existing transactions within the mempool through the Replace-By-Fee (RBF) signaling policy."`
	RelayNonStd         bool     `yaml:"relay_non_std" long:"relaynonstd" description:"Relay non-standard transactions regardless of the default settings for the active network."`
	FreeTxRelayLimit    float64  `yaml:"free_tx_relay_limit" long:"limitfreerelay" description:"Limit relay of transactions with no transaction fee to the given amount in thousands of bytes per minute"`
	MaxOrphanTxs        int      `yaml:"max_orphan_txs" long:"maxorphantx" description:"Max number of orphan transactions to keep in memory"`
	MinRelayTxFee       float64  `yaml:"min_relay_tx_fee" long:"minrelaytxfee" description:"The minimum transaction fee in BTC/kB to be considered a non-zero fee."`
	MinRelayTxFeeValues btcutil.Amount
	NoCFilters          bool     `yaml:"no_c_filters" long:"nocfilters" description:"Disable committed filtering (CF) support"`
	DisableCheckpoints  bool     `yaml:"disable_checkpoints" long:"nocheckpoints" description:"Disable built-in checkpoints.  Don't do this unless you know what you're doing."`
	MiningAddresses     []string `yaml:"mining_addresses"`
}

func (cfg *ChainRuntimeConfig) ParseMiningAddresses(params *chain.Params) ([]btcutil.Address, error) {
	miningAddrs := make([]btcutil.Address, 0, len(cfg.MiningAddresses))
	for _, address := range cfg.MiningAddresses {
		addr, err := btcutil.DecodeAddress(address, params)
		if err != nil {
			return nil, err
		}

		miningAddrs = append(miningAddrs, addr)
	}
	return miningAddrs, nil
}

type ChainActor struct {

	// StartupTime is the unix timestamp for when the Server that is hosting
	// the RPC Server started.
	StartupTime int64

	// todo(mike): ConnMgr SyncMgr ShardsMgr are nil need to refactor it
	// ConnMgr defines the connection manager for the RPC Server to use.  It
	// provides the RPC Server with a means to do things such as add,
	// remove, connect, disconnect, and query peers as well as other
	// connection-related data and tasks.
	ConnMgr rpcserverConnManager

	// SyncMgr defines the sync manager for the RPC Server to use.
	SyncMgr rpcserverSyncManager

	// ShardsMgr provides ability to manipulate running shards.
	ShardsMgr ShardManager

	// These fields allow the RPC Server to interface with the local block
	// BlockChain data and state.
	TimeSource  blockchain.MedianTimeSource
	ChainParams *chain.Params
	DB          database.DB

	// TxMemPool defines the transaction memory pool to interact with.
	TxMemPool *mempool.TxPool

	// These fields allow the RPC Server to interface with mining.
	//
	// Generator produces block templates and the CPUMiner solves them using
	// the CPU.  CPU mining is typically only useful for test purposes when
	// doing regression or simulation testing.
	Generator *mining.BlkTmplGenerator
	CPUMiner  *cpuminer.CPUMiner

	// These fields define any optional indexes the RPC Server can make use
	// of to provide additional data when queried.
	TxIndex   *indexers.TxIndex
	AddrIndex *indexers.AddrIndex
	CfIndex   *indexers.CfIndex

	// The fee estimator keeps track of how long transactions are left in
	// the mempool before they are mined into blocks.
	FeeEstimator *mempool.FeeEstimator
	MiningAddrs  []btcutil.Address

	SigCache    *txscript.SigCache
	HashCache   *txscript.HashCache
	SyncManager *netsync.SyncManager
	BlockChain  *blockchain.BlockChain

	IChain chain.IChain
	logger *zap.Logger
	config *ChainRuntimeConfig
}

func NewChainActor(ctx context.Context, cfg ChainRuntimeConfig, iChain chain.IChain, db database.DB, log *zap.Logger) (*ChainActor, error) {
	chainActor := &ChainActor{
		IChain:      iChain,
		ChainParams: iChain.Params(),
		DB:          db,
		TimeSource:  blockchain.NewMedianTime(),
		SigCache:    txscript.NewSigCache(cfg.SigCacheMaxSize),
		HashCache:   txscript.NewHashCache(cfg.SigCacheMaxSize),
		logger:      log,
		config:      &cfg,
		// ConnMgr:      &RPCConnManager{Server: beaconCtl.p2pServer},
		// SyncMgr:      &RPCSyncMgr{Server: beaconCtl.p2pServer, SyncMgr: beaconCtl.p2pServer.SyncManager},
	}

	var err error

	// Search for a FeeEstimator state in the database. If none can be found
	// or if it cannot be loaded, create a new one.
	if err = db.Update(chainActor.DBUpdateCallback); err != nil {
		return nil, err
	}

	if err = chainActor.initBlockchainAndMempool(ctx, cfg); err != nil {
		return nil, err
	}

	chainActor.SyncManager, err = netsync.New(&netsync.Config{
		// todo(fix me)
		// PeerNotifier: chainActor,
		Chain:              chainActor.BlockChain,
		TxMemPool:          chainActor.TxMemPool,
		ChainParams:        chainActor.ChainParams,
		DisableCheckpoints: cfg.DisableCheckpoints,
		MaxPeers:           cfg.MaxPeers,
		FeeEstimator:       chainActor.FeeEstimator,
	})
	if err != nil {
		return nil, err
	}

	// Create the mining policy and block template generator based on the
	// configuration options.
	//
	// NOTE: The CPU miner relies on the mempool, so the mempool has to be
	// created before calling the function to create the CPU miner.
	policy := mining.Policy{
		BlockMinWeight:    cfg.BlockMinWeight,
		BlockMaxWeight:    cfg.BlockMaxWeight,
		BlockMinSize:      cfg.BlockMinSize,
		BlockMaxSize:      cfg.BlockMaxSize,
		BlockPrioritySize: cfg.BlockPrioritySize,
		TxMinFreeFee:      cfg.MinRelayTxFeeValues,
	}
	chainActor.Generator = mining.NewBlkTmplGenerator(&policy,
		chainActor.ChainParams, chainActor.TxMemPool, chainActor.BlockChain, chainActor.TimeSource,
		chainActor.SigCache, chainActor.HashCache)

	chainActor.MiningAddrs, err = cfg.ParseMiningAddresses(chainActor.IChain.Params())
	if err != nil {
		return nil, err
	}

	return chainActor, nil
}

func (chainActor *ChainActor) DBUpdateCallback(tx database.Tx) error {
	metadata := tx.Metadata()
	feeEstimationData := metadata.Get(mempool.EstimateFeeDatabaseKey)
	if feeEstimationData != nil {
		// delete it from the database so that we don't try to restore the
		// same thing again somehow.
		metadata.Delete(mempool.EstimateFeeDatabaseKey)

		// If there is an error, log it and make a new fee estimator.
		var err error
		chainActor.FeeEstimator, err = mempool.RestoreFeeEstimator(feeEstimationData)

		if err != nil {
			chainActor.logger.Error("Failed to restore fee estimator", zap.Error(err))
		}
	}

	return nil
}

func (chainActor *ChainActor) Config() *ChainRuntimeConfig {
	return chainActor.config
}

func (chainActor *ChainActor) Log() *zap.Logger {
	return chainActor.logger
}

func (chainActor *ChainActor) InitCPUMiner(connectedCount func() int32) *cpuminer.CPUMiner {
	chainActor.CPUMiner = cpuminer.New(&cpuminer.Config{
		ChainParams:            chainActor.ChainParams,
		BlockTemplateGenerator: chainActor.Generator,
		MiningAddrs:            chainActor.MiningAddrs,

		ProcessBlock:   chainActor.SyncManager.ProcessBlock,
		IsCurrent:      chainActor.SyncManager.IsCurrent,
		ConnectedCount: connectedCount,
	}, chainActor.logger)

	return chainActor.CPUMiner
}

func (chainActor *ChainActor) initBlockchainAndMempool(ctx context.Context, cfg ChainRuntimeConfig) error {
	indexManager, checkpoints := chainActor.initIndexes(cfg)

	// Create a new blockchain instance with the appropriate configuration.
	var err error
	chainActor.BlockChain, err = blockchain.New(&blockchain.Config{
		DB:           chainActor.DB,
		Interrupt:    ctx.Done(),
		ChainParams:  chainActor.ChainParams,
		Checkpoints:  checkpoints,
		IndexManager: indexManager,
		TimeSource:   chainActor.TimeSource,
		SigCache:     chainActor.SigCache,
		HashCache:    chainActor.HashCache,
		Chain:        chainActor.IChain,
	})
	if err != nil {
		return err
	}

	// If no FeeEstimator has been found, or if the one that has been found
	// is behind somehow, create a new one and start over.
	if chainActor.FeeEstimator == nil || chainActor.FeeEstimator.LastKnownHeight() != chainActor.BlockChain.BestSnapshot().Height {
		chainActor.FeeEstimator = mempool.NewFeeEstimator(
			mempool.DefaultEstimateFeeMaxRollback,
			mempool.DefaultEstimateFeeMinRegisteredBlocks)
	}

	txC := mempool.Config{
		Policy: mempool.Policy{
			DisableRelayPriority: cfg.NoRelayPriority,
			AcceptNonStd:         cfg.RelayNonStd,
			FreeTxRelayLimit:     cfg.FreeTxRelayLimit,
			MaxOrphanTxs:         cfg.MaxOrphanTxs,
			MaxOrphanTxSize:      defaultMaxOrphanTxSize,
			MaxSigOpCostPerTx:    blockchain.MaxBlockSigOpsCost / 4,
			MinRelayTxFee:        cfg.MinRelayTxFeeValues,
			MaxTxVersion:         2,
			RejectReplacement:    cfg.RejectReplacement,
		},
		ChainParams:    chainActor.ChainParams,
		FetchUtxoView:  chainActor.BlockChain.FetchUtxoView,
		BestHeight:     func() int32 { return chainActor.BlockChain.BestSnapshot().Height },
		MedianTimePast: func() time.Time { return chainActor.BlockChain.BestSnapshot().MedianTime },
		CalcSequenceLock: func(tx *btcutil.Tx, view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return chainActor.BlockChain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: chainActor.BlockChain.IsDeploymentActive,
		SigCache:           chainActor.SigCache,
		HashCache:          chainActor.HashCache,
		AddrIndex:          chainActor.AddrIndex,
		FeeEstimator:       chainActor.FeeEstimator,
	}

	chainActor.TxMemPool = mempool.New(&txC)
	return nil
}

func (chainActor *ChainActor) initIndexes(cfg ChainRuntimeConfig) (blockchain.IndexManager, []chain.Checkpoint) {
	// Create the transaction and address indexes if needed.
	//
	// CAUTION: the txindex needs to be first in the indexes array because
	// the addrindex uses data from the txindex during catchup.  If the
	// addrindex is run first, it may not have the transactions from the
	// current block indexed.
	var indexes []indexers.Indexer
	if cfg.TxIndex || cfg.AddrIndex {
		// Enable transaction index if address index is enabled since it
		// requires it.
		if !cfg.TxIndex {
			chainActor.logger.Info("Transaction index enabled because it " +
				"is required by the address index")
			cfg.TxIndex = true
		} else {
			chainActor.logger.Info("Transaction index is enabled")
		}

		chainActor.TxIndex = indexers.NewTxIndex(chainActor.DB)
		indexes = append(indexes, chainActor.TxIndex)
	}
	if cfg.AddrIndex {
		chainActor.logger.Info("Address index is enabled")
		chainActor.AddrIndex = indexers.NewAddrIndex(chainActor.DB, chainActor.IChain.Params())
		indexes = append(indexes, chainActor.AddrIndex)
	}
	if !cfg.NoCFilters {
		chainActor.logger.Info("Committed filter index is enabled")
		chainActor.CfIndex = indexers.NewCfIndex(chainActor.DB, chainActor.IChain.Params())
		indexes = append(indexes, chainActor.CfIndex)
	}

	// Create an index manager if any of the optional indexes are enabled.
	var indexManager blockchain.IndexManager
	if len(indexes) > 0 {
		indexManager = indexers.NewManager(chainActor.DB, indexes)
	}

	// Merge given checkpoints with the default ones unless they are disabled.
	var checkpoints []chain.Checkpoint
	if !cfg.DisableCheckpoints {
		// checkpoints = mergeCheckpoints(chainActor.ChainParams.Checkpoints, cfg.AddCheckpoints)
	}

	return indexManager, checkpoints
}
