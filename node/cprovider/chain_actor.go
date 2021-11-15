// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.+

package cprovider

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain/indexers"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx/btcd"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const defaultMaxOrphanTxSize = 100000

type ChainRuntimeConfig struct {
	SigCacheMaxSize    uint     `yaml:"sig_cache_max_size" toml:"sig_cache_max_size" description:"The maximum number of entries in the signature verification cache"`
	AddCheckpoints     []string `yaml:"add_checkpoints" toml:"add_checkpoints" description:"Add a custom checkpoint.  Format: '<height>:<hash>'"`
	AddrIndex          bool     `yaml:"addr_index" toml:"addr_index" description:"Maintain a full address-based transaction index which makes the searchrawtransactions RPC available"`
	MaxPeers           int      `yaml:"max_peers" toml:"max_peers" description:"Max number of inbound and outbound peers"`
	BlockMaxSize       uint32   `yaml:"block_max_size" toml:"block_max_size" description:"Maximum block size in bytes to be used when creating a block"`
	BlockMinSize       uint32   `yaml:"block_min_size" toml:"block_min_size" description:"Mininum block size in bytes to be used when creating a block"`
	BlockMaxWeight     uint32   `yaml:"block_max_weight" toml:"block_max_weight" description:"Maximum block weight to be used when creating a block"`
	BlockMinWeight     uint32   `yaml:"block_min_weight" toml:"block_min_weight" description:"Mininum block weight to be used when creating a block"`
	BlockPrioritySize  uint32   `yaml:"block_priority_size" toml:"block_priority_size" description:"Size in bytes for high-priority/low-fee transactions when creating a block"`
	TxIndex            bool     `yaml:"tx_index" toml:"tx_index" long:"txindex" description:"Maintain a full hash-based transaction index which makes all transactions available via the getrawtransaction RPC"`
	NoRelayPriority    bool     `yaml:"no_relay_priority" toml:"no_relay_priority" description:"Do not require free or low-fee transactions to have high priority for relaying"`
	RejectReplacement  bool     `yaml:"reject_replacement" toml:"reject_replacement" description:"Reject transactions that attempt to replace existing transactions within the mempool through the Replace-By-Fee (RBF) signaling policy."`
	RelayNonStd        bool     `yaml:"relay_non_std" toml:"relay_non_std" description:"Relay non-standard transactions regardless of the default settings for the active network."`
	FreeTxRelayLimit   float64  `yaml:"free_tx_relay_limit" toml:"free_tx_relay_limit" description:"Limit relay of transactions with no transaction fee to the given amount in thousands of bytes per minute"`
	MaxOrphanTxs       int      `yaml:"max_orphan_txs" toml:"max_orphan_txs" description:"Max number of orphan transactions to keep in memory"`
	MinRelayTxFee      int64    `yaml:"min_relay_tx_fee" toml:"min_relay_tx_fee" description:"The minimum transaction fee in satoshi/kB to be considered a non-zero fee."`
	NoCFilters         bool     `yaml:"no_c_filters" toml:"no_c_filters" description:"Disable committed filtering (CF) support"`
	DisableCheckpoints bool     `yaml:"disable_checkpoints" toml:"disable_checkpoints" description:"Disable built-in checkpoints.  Don't do this unless you know what you're doing."`
	MiningAddresses    []string `yaml:"mining_addresses" toml:"mining_addresses"`
	AutoExpand         bool     `yaml:"auto_expand" toml:"auto_expand"`
	ExpansionRule      int32    `yaml:"expansion_rule" toml:"expansion_rule"`
	ExpansionLimit     int32    `yaml:"expansion_limit" toml:"expansion_limit"`
}

func (cfg *ChainRuntimeConfig) ParseMiningAddresses(params *chaincfg.Params) ([]jaxutil.Address, error) {
	miningAddrs := make([]jaxutil.Address, 0, len(cfg.MiningAddresses))
	for _, address := range cfg.MiningAddresses {
		addr, err := jaxutil.DecodeAddress(address, params)
		if err != nil {
			return nil, err
		}

		miningAddrs = append(miningAddrs, addr)
	}
	return miningAddrs, nil
}

type ChainProvider struct {
	// StartupTime is the unix timestamp for when the Server that is hosting
	// the RPC Server started.
	StartupTime int64
	ChainCtx    chainctx.IChainCtx
	ChainParams *chaincfg.Params

	DB database.DB
	// These fields allow the RPC Server to interface with the local block
	// BlockChain data and state.
	TimeSource chaindata.MedianTimeSource

	// TxMemPool defines the transaction memory pool to interact with.
	TxMemPool *mempool.TxPool

	// These fields define any optional indexes the RPC Server can make use
	// of to provide additional data when queried.
	TxIndex       *indexers.TxIndex
	AddrIndex     *indexers.AddrIndex
	CfIndex       *indexers.CfIndex
	OrphanTxIndex *indexers.OrphanTxIndex

	// The fee estimator keeps track of how long transactions are left in
	// the mempool before they are mined into blocks.
	FeeEstimator *mempool.FeeEstimator
	MiningAddrs  []jaxutil.Address

	SigCache    *txscript.SigCache
	HashCache   *txscript.HashCache
	SyncManager *netsync.SyncManager
	blockChain  *blockchain.BlockChain

	logger zerolog.Logger
	config *ChainRuntimeConfig

	blockTmplGenerator *mining.BlkTmplGenerator
	gbtWorkState       *mining.GBTWorkState
}

func NewChainProvider(ctx context.Context, cfg ChainRuntimeConfig, chainCtx chainctx.IChainCtx,
	blockGen chaindata.ChainBlockGenerator, db database.DB, log zerolog.Logger) (*ChainProvider, error) {
	var err error

	chainProvider := &ChainProvider{
		ChainCtx:    chainCtx,
		ChainParams: chainCtx.Params(),
		DB:          db,
		TimeSource:  chaindata.NewMedianTime(),
		SigCache:    txscript.NewSigCache(cfg.SigCacheMaxSize),
		HashCache:   txscript.NewHashCache(cfg.SigCacheMaxSize),
		logger:      log,
		config:      &cfg,
	}

	chainProvider.MiningAddrs, err = cfg.ParseMiningAddresses(chainProvider.ChainCtx.Params())
	if err != nil {
		return nil, err
	}

	// Search for a FeeEstimator state in the database. If none can be found
	// or if it cannot be loaded, create a new one.
	if err = db.Update(chainProvider.DBUpdateCallback); err != nil {
		return nil, err
	}

	if err = chainProvider.initBlockchainAndMempool(ctx, cfg, blockGen); err != nil {
		return nil, err
	}

	chainProvider.initBlkTmplGenerator()

	return chainProvider, nil
}

func (chainProvider *ChainProvider) SetP2PProvider(p2pProvider netsync.PeerNotifier) (err error) {
	chainProvider.SyncManager, err = netsync.New(&netsync.Config{
		PeerNotifier:       p2pProvider,
		Chain:              chainProvider.blockChain,
		TxMemPool:          chainProvider.TxMemPool,
		ChainParams:        chainProvider.ChainParams,
		DisableCheckpoints: chainProvider.config.DisableCheckpoints,
		MaxPeers:           chainProvider.config.MaxPeers,
		FeeEstimator:       chainProvider.FeeEstimator,
	})

	if err != nil {
		return err
	}
	return err
}

func (chainProvider *ChainProvider) DBUpdateCallback(tx database.Tx) error {
	metadata := tx.Metadata()
	feeEstimationData := metadata.Get(mempool.EstimateFeeDatabaseKey)
	if feeEstimationData != nil {
		// delete it from the database so that we don't try to restore the
		// same thing again somehow.
		_ = metadata.Delete(mempool.EstimateFeeDatabaseKey)

		// If there is an error, log it and make a new fee estimator.
		var err error
		chainProvider.FeeEstimator, err = mempool.RestoreFeeEstimator(feeEstimationData)
		if err != nil {
			chainProvider.logger.Error().Err(err).Msg("Failed to restore fee estimator")
		}
	}

	return nil
}

func (chainProvider *ChainProvider) Config() *ChainRuntimeConfig {
	return chainProvider.config
}

func (chainProvider *ChainProvider) Log() zerolog.Logger {
	return chainProvider.logger
}

func (chainProvider *ChainProvider) initBlkTmplGenerator() {
	minRelayTxFee := mempool.MinRelayFeeAmount(chainProvider.ChainCtx.IsBeacon())
	if chainProvider.config.MinRelayTxFee > 1 {
		minRelayTxFee = jaxutil.Amount(chainProvider.config.MinRelayTxFee)
	}

	// Create the mining policy and block template generator based on the
	// configuration options.
	policy := mining.Policy{
		BlockMinWeight:    chainProvider.config.BlockMinWeight,
		BlockMaxWeight:    chainProvider.config.BlockMaxWeight,
		BlockMinSize:      chainProvider.config.BlockMinSize,
		BlockMaxSize:      chainProvider.config.BlockMaxSize,
		BlockPrioritySize: chainProvider.config.BlockPrioritySize,
		TxMinFreeFee:      minRelayTxFee,
	}

	chainProvider.blockTmplGenerator = mining.NewBlkTmplGenerator(&policy,
		chainProvider.ChainCtx, chainProvider.TxMemPool, chainProvider.blockChain)

	chainProvider.gbtWorkState = mining.NewGbtWorkState(chainProvider.TimeSource,
		chainProvider.blockTmplGenerator, chainProvider.logger)
}

func (chainProvider *ChainProvider) BlkTmplGenerator() *mining.BlkTmplGenerator {
	return chainProvider.blockTmplGenerator
}

func (chainProvider *ChainProvider) GbtWorkState() *mining.GBTWorkState {
	return chainProvider.gbtWorkState
}

func (chainProvider *ChainProvider) Stats() map[string]float64 {
	shards, _ := chainProvider.ShardCount()
	snapshot := chainProvider.blockChain.BestSnapshot()

	return map[string]float64{
		"shards":             float64(shards),
		"height":             float64(snapshot.Height),
		"size":               float64(snapshot.BlockSize),
		"txs":                float64(snapshot.NumTxns),
		"median_time":        float64(snapshot.MedianTime.Unix()),
		"bits":               float64(snapshot.Bits),
		"total_transactions": float64(snapshot.TotalTxns),
		"txs_in_mempool":     float64(chainProvider.TxMemPool.Count()),
	}
}

func (chainProvider *ChainProvider) BlockChain() *blockchain.BlockChain {
	return chainProvider.blockChain
}

func (chainProvider *ChainProvider) MiningAddresses() []jaxutil.Address {
	return chainProvider.MiningAddrs
}

func (chainProvider *ChainProvider) BlockTemplate(useCoinbaseValue bool, burnReward int) (chaindata.BlockTemplate, error) {
	return chainProvider.gbtWorkState.BlockTemplate(chainProvider, useCoinbaseValue, burnReward)
}

func (chainProvider *ChainProvider) ShardCount() (uint32, error) {
	return chainProvider.BlockChain().ShardCount()
}

func (chainProvider *ChainProvider) BestSnapshot() *chaindata.BestState {
	return chainProvider.BlockChain().BestSnapshot()
}

func (chainProvider *ChainProvider) CalcKForHeight(height int32) uint32 {
	return chainProvider.BlockChain().CalcKForHeight(height)
}

func (chainProvider *ChainProvider) BTC() *btcd.BlockProvider {
	// todo
	return nil
}

// nolint: staticcheck
func (chainProvider *ChainProvider) initBlockchainAndMempool(ctx context.Context, cfg ChainRuntimeConfig,
	blockGen chaindata.ChainBlockGenerator) error {
	indexManager, checkpoints := chainProvider.initIndexes(cfg)

	// Create a new blockchain instance with the appropriate configuration.
	var err error
	chainProvider.blockChain, err = blockchain.New(&blockchain.Config{
		DB:           chainProvider.DB,
		Interrupt:    ctx.Done(),
		ChainParams:  chainProvider.ChainParams,
		Checkpoints:  checkpoints,
		IndexManager: indexManager,
		TimeSource:   chainProvider.TimeSource,
		SigCache:     chainProvider.SigCache,
		HashCache:    chainProvider.HashCache,
		ChainCtx:     chainProvider.ChainCtx,
		BlockGen:     blockGen,
	})
	if err != nil {
		return err
	}

	// If no FeeEstimator has been found, or if the one that has been found
	// is behind somehow, create a new one and start over.
	if chainProvider.FeeEstimator == nil || chainProvider.FeeEstimator.LastKnownHeight() != chainProvider.blockChain.BestSnapshot().Height {
		chainProvider.FeeEstimator = mempool.NewFeeEstimator(
			mempool.DefaultEstimateFeeMaxRollback,
			mempool.DefaultEstimateFeeMinRegisteredBlocks)
	}

	minRelayTxFee := mempool.MinRelayFeeAmount(chainProvider.ChainCtx.IsBeacon())
	if chainProvider.config.MinRelayTxFee > 1 {
		minRelayTxFee = jaxutil.Amount(chainProvider.config.MinRelayTxFee)
	}

	txC := mempool.Config{
		Policy: mempool.Policy{
			DisableRelayPriority: cfg.NoRelayPriority,
			AcceptNonStd:         cfg.RelayNonStd,
			FreeTxRelayLimit:     cfg.FreeTxRelayLimit,
			MaxOrphanTxs:         cfg.MaxOrphanTxs,
			MaxOrphanTxSize:      defaultMaxOrphanTxSize,
			MaxSigOpCostPerTx:    chaindata.MaxBlockSigOpsCost / 4,
			MinRelayTxFee:        minRelayTxFee,
			MaxTxVersion:         wire.MaxTxVersion,
			RejectReplacement:    cfg.RejectReplacement,
		},
		ChainParams:    chainProvider.ChainParams,
		FetchUtxoView:  chainProvider.blockChain.FetchUtxoView,
		BestHeight:     func() int32 { return chainProvider.blockChain.BestSnapshot().Height },
		MedianTimePast: func() time.Time { return chainProvider.blockChain.BestSnapshot().MedianTime },
		CalcSequenceLock: func(tx *jaxutil.Tx, view *chaindata.UtxoViewpoint) (*chaindata.SequenceLock, error) {
			return chainProvider.blockChain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: chainProvider.blockChain.IsDeploymentActive,
		SigCache:           chainProvider.SigCache,
		HashCache:          chainProvider.HashCache,
		AddrIndex:          chainProvider.AddrIndex,
		FeeEstimator:       chainProvider.FeeEstimator,
	}

	chainProvider.TxMemPool = mempool.New(&txC)
	return nil
}

// nolint: unparam
func (chainProvider *ChainProvider) initIndexes(cfg ChainRuntimeConfig) (blockchain.IndexManager, []chaincfg.Checkpoint) {
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
			chainProvider.logger.Info().Msg("Transaction index enabled because it " +
				"is required by the address index")
			cfg.TxIndex = true
		} else {
			chainProvider.logger.Info().Msg("Transaction index is enabled")
		}

		chainProvider.TxIndex = indexers.NewTxIndex(chainProvider.DB)
		chainProvider.OrphanTxIndex = indexers.NewOrphanTxIndex(chainProvider.DB)
		indexes = append(indexes, chainProvider.TxIndex, chainProvider.OrphanTxIndex)
	}
	if cfg.AddrIndex {
		chainProvider.logger.Info().Msg("Address index is enabled")
		chainProvider.AddrIndex = indexers.NewAddrIndex(chainProvider.DB, chainProvider.ChainCtx.Params())
		indexes = append(indexes, chainProvider.AddrIndex)
	}

	if !cfg.NoCFilters {
		chainProvider.logger.Info().Msg("Committed filter index is enabled")
		chainProvider.CfIndex = indexers.NewCfIndex(chainProvider.DB, chainProvider.ChainCtx.Params())
		indexes = append(indexes, chainProvider.CfIndex)
	}

	// Create an index manager if any of the optional indexes are enabled.
	var indexManager blockchain.IndexManager
	if len(indexes) > 0 {
		indexManager = indexers.NewManager(chainProvider.DB, indexes, chainProvider.ChainCtx.Name())
	}

	// Merge given checkpoints with the default ones unless they are disabled.
	var checkpoints []chaincfg.Checkpoint
	// if !cfg.DisableCheckpoints {
	//	checkpoints = mergeCheckpoints(chainProvider.ChainParams.Checkpoints, cfg.AddCheckpoints)
	// }

	return indexManager, checkpoints
}
