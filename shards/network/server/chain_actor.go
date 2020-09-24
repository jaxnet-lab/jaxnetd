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

type ChainActor struct {

	// StartupTime is the unix timestamp for when the Server that is hosting
	// the RPC Server started.
	StartupTime int64

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
	Chain       *blockchain.BlockChain
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

	logger *zap.Logger
	IChain chain.IChain
}

func NewChainActor(ctx context.Context, cfg ChainRuntimeConfig, log *zap.Logger, iChain chain.IChain, db database.DB) (*ChainActor, error) {
	s := &ChainActor{
		IChain:      iChain,
		ChainParams: cfg.ChainParams(),
		DB:          db,
		TimeSource:  blockchain.NewMedianTime(),
		SigCache:    txscript.NewSigCache(cfg.SigCacheMaxSize),
		HashCache:   txscript.NewHashCache(cfg.SigCacheMaxSize),
		logger:      log,
	}

	var err error

	// Search for a FeeEstimator state in the database. If none can be found
	// or if it cannot be loaded, create a new one.
	if err = db.Update(s.DBUpdateCallback); err != nil {
		return nil, err
	}

	if err = s.initBlockchainAndMempool(ctx, cfg); err != nil {
		return nil, err
	}

	s.SyncManager, err = netsync.New(&netsync.Config{
		// todo(fix me)
		// PeerNotifier: s,
		Chain:              s.BlockChain,
		TxMemPool:          s.TxMemPool,
		ChainParams:        s.ChainParams,
		DisableCheckpoints: cfg.DisableCheckpoints,
		MaxPeers:           cfg.MaxPeers,
		FeeEstimator:       s.FeeEstimator,
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
	s.Generator = mining.NewBlkTmplGenerator(&policy,
		s.ChainParams, s.TxMemPool, s.BlockChain, s.TimeSource,
		s.SigCache, s.HashCache)

	return s, nil
}

func (s *ChainActor) DBUpdateCallback(tx database.Tx) error {
	metadata := tx.Metadata()
	feeEstimationData := metadata.Get(mempool.EstimateFeeDatabaseKey)
	if feeEstimationData != nil {
		// delete it from the database so that we don't try to restore the
		// same thing again somehow.
		metadata.Delete(mempool.EstimateFeeDatabaseKey)

		// If there is an error, log it and make a new fee estimator.
		var err error
		s.FeeEstimator, err = mempool.RestoreFeeEstimator(feeEstimationData)

		if err != nil {
			s.logger.Error("Failed to restore fee estimator", zap.Error(err))
		}
	}

	return nil
}

func (s *ChainActor) initBlockchainAndMempool(ctx context.Context, cfg ChainRuntimeConfig) error {
	indexManager, checkpoints := s.initIndexes(cfg)

	// Create a new blockchain instance with the appropriate configuration.
	var err error
	s.BlockChain, err = blockchain.New(&blockchain.Config{
		DB:           s.DB,
		Interrupt:    ctx.Done(),
		ChainParams:  s.ChainParams,
		Checkpoints:  checkpoints,
		IndexManager: indexManager,
		TimeSource:   s.TimeSource,
		SigCache:     s.SigCache,
		HashCache:    s.HashCache,
		Chain:        s.IChain,
	})
	if err != nil {
		return err
	}

	// If no FeeEstimator has been found, or if the one that has been found
	// is behind somehow, create a new one and start over.
	if s.FeeEstimator == nil || s.FeeEstimator.LastKnownHeight() != s.BlockChain.BestSnapshot().Height {
		s.FeeEstimator = mempool.NewFeeEstimator(
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
		ChainParams:    s.ChainParams,
		FetchUtxoView:  s.BlockChain.FetchUtxoView,
		BestHeight:     func() int32 { return s.BlockChain.BestSnapshot().Height },
		MedianTimePast: func() time.Time { return s.BlockChain.BestSnapshot().MedianTime },
		CalcSequenceLock: func(tx *btcutil.Tx, view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return s.BlockChain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: s.BlockChain.IsDeploymentActive,
		SigCache:           s.SigCache,
		HashCache:          s.HashCache,
		AddrIndex:          s.AddrIndex,
		FeeEstimator:       s.FeeEstimator,
	}

	s.TxMemPool = mempool.New(&txC)
	return nil
}

func (s *ChainActor) initIndexes(cfg ChainRuntimeConfig) (blockchain.IndexManager, []chain.Checkpoint) {
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
			s.logger.Info("Transaction index enabled because it " +
				"is required by the address index")
			cfg.TxIndex = true
		} else {
			s.logger.Info("Transaction index is enabled")
		}

		s.TxIndex = indexers.NewTxIndex(s.DB)
		indexes = append(indexes, s.TxIndex)
	}
	if cfg.AddrIndex {
		s.logger.Info("Address index is enabled")
		s.AddrIndex = indexers.NewAddrIndex(s.DB, cfg.ChainParams())
		indexes = append(indexes, s.AddrIndex)
	}
	if !cfg.NoCFilters {
		s.logger.Info("Committed filter index is enabled")
		s.CfIndex = indexers.NewCfIndex(s.DB, cfg.ChainParams())
		indexes = append(indexes, s.CfIndex)
	}

	// Create an index manager if any of the optional indexes are enabled.
	var indexManager blockchain.IndexManager
	if len(indexes) > 0 {
		indexManager = indexers.NewManager(s.DB, indexes)
	}

	// Merge given checkpoints with the default ones unless they are disabled.
	var checkpoints []chain.Checkpoint
	if !cfg.DisableCheckpoints {
		// checkpoints = mergeCheckpoints(s.ChainParams.Checkpoints, cfg.AddCheckpoints)
	}

	return indexManager, checkpoints
}
