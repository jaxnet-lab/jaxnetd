// Copyright (c) 2013-2018 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/mmr"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	// maxOrphanBlocks is the maximum number of orphan blocks that can be
	// queued.
	maxOrphanBlocks = 100
)

// BlockLocator is used to help locate a specific block.  The algorithm for
// building the block locator is to add the hashes in reverse order until
// the genesis block is reached.  In order to keep the list of locator hashes
// to a reasonable number of entries, first the most recent previous 12 block
// hashes are added, then the step is doubled each loop iteration to
// exponentially decrease the number of hashes as a function of the distance
// from the block being located.
//
// For example, assume a block chain with a side chain as depicted below:
//
//	genesis -> 1 -> 2 -> ... -> 15 -> 16  -> 17  -> 18
//	                              \-> 16a -> 17a
//
// The block locator for block 17a would be the hashes of blocks:
// [17a 16a 15 14 13 12 11 10 9 8 7 6 4 genesis]
type BlockLocator []*wire.BlockLocatorMeta

// orphanBlock represents a block that we don't yet have the parent for.  It
// is a normal block plus an expiration time to prevent caching the orphan
// forever.
type orphanBlock struct {
	block      *jaxutil.Block
	actualMMR  chainhash.Hash
	expiration time.Time
}

// IndexManager provides a generic interface that the is called when blocks are
// connected and disconnected to and from the tip of the main chain for the
// purpose of supporting optional indexes.
type IndexManager interface {
	// Init is invoked during chain initialize in order to allow the index
	// manager to initialize itself and any indexes it is managing.  The
	// channel parameter specifies a channel the caller can close to signal
	// that the process should be interrupted.  It can be nil if that
	// behavior is not desired.
	Init(*BlockChain, <-chan struct{}) error

	// ConnectBlock is invoked when a new block has been connected to the
	// main chain. The set of output spent within a block is also passed in
	// so indexers can access the previous output scripts input spent if
	// required.
	ConnectBlock(database.Tx, *jaxutil.Block, chainhash.Hash, []chaindata.SpentTxOut) error

	// DisconnectBlock is invoked when a block has been disconnected from
	// the main chain. The set of outputs scripts that were spent within
	// this block is also returned so indexers can clean up the prior index
	// state for this block.
	DisconnectBlock(database.Tx, *jaxutil.Block, chainhash.Hash, []chaindata.SpentTxOut) error
}

// Config is a descriptor which specifies the blockchain instance configuration.
type Config struct {
	// todo: combine this fields

	// DB defines the database which houses the blocks and will be used to
	// store all metadata created by this package such as the utxo set.
	//
	// This field is required.
	DB database.DB

	// ChainParams identifies which chain parameters the chain is associated
	// with.
	//
	// This field is required.
	ChainParams *chaincfg.Params
	ChainCtx    chainctx.IChainCtx
	BlockGen    chaindata.ChainBlockGenerator
	// ------ ------ ------ ------ ------

	// Interrupt specifies a channel the caller can close to signal that
	// long running operations, such as catching up indexes or performing
	// database migrations, should be interrupted.
	//
	// This field can be nil if the caller does not desire the behavior.
	Interrupt <-chan struct{}

	// Checkpoints hold caller-defined checkpoints that should be added to
	// the default checkpoints in ChainParams.  Checkpoints must be sorted
	// by height.
	//
	// This field can be nil if the caller does not wish to specify any
	// checkpoints.
	Checkpoints []chaincfg.Checkpoint

	// TimeSource defines the median time source to use for things such as
	// block processing and determining whether or not the chain is current.
	//
	// The caller is expected to keep a reference to the time source as well
	// and add time samples from other peers on the network so the local
	// time is adjusted to be in agreement with other peers.
	TimeSource chaindata.MedianTimeSource

	// SigCache defines a signature cache to use when when validating
	// signatures.  This is typically most useful when individual
	// transactions are already being validated prior to their inclusion in
	// a block such as what is usually done via a transaction memory pool.
	//
	// This field can be nil if the caller is not interested in using a
	// signature cache.
	SigCache *txscript.SigCache

	// IndexManager defines an index manager to use when initializing the
	// chain and connecting and disconnecting blocks.
	//
	// This field can be nil if the caller does not wish to make use of an
	// index manager.
	IndexManager IndexManager

	// HashCache defines a transaction hash mid-state cache to use when
	// validating transactions. This cache has the potential to greatly
	// speed up transaction validation as re-using the pre-calculated
	// mid-state eliminates the O(N^2) validation complexity due to the
	// SigHashAll flag.
	//
	// This field can be nil if the caller is not interested in using a
	// signature cache.
	HashCache *txscript.HashCache

	DBFullRescan     bool
	TryToRepairState bool
}

// New returns a BlockChain instance using the provided configuration details.
func New(config *Config) (*BlockChain, error) {
	// Enforce required config fields.
	if config.DB == nil {
		return nil, chaindata.AssertError("blockchain.New database is nil")
	}
	if config.ChainParams == nil {
		return nil, chaindata.AssertError("blockchain.New chain parameters nil")
	}
	if config.TimeSource == nil {
		return nil, chaindata.AssertError("blockchain.New timesource is nil")
	}

	// Generate a checkpoint by height map from the provided checkpoints
	// and assert the provided checkpoints are sorted by height as required.
	var checkpointsByHeight map[int32]*chaincfg.Checkpoint
	var prevCheckpointHeight int32
	if len(config.Checkpoints) > 0 {
		checkpointsByHeight = make(map[int32]*chaincfg.Checkpoint)
		for i := range config.Checkpoints {
			checkpoint := &config.Checkpoints[i]
			if checkpoint.Height <= prevCheckpointHeight {
				return nil, chaindata.AssertError("blockchain.New " +
					"checkpoints are not sorted by height")
			}

			checkpointsByHeight[checkpoint.Height] = checkpoint
			prevCheckpointHeight = checkpoint.Height
		}
	}

	params := config.ChainCtx.Params()

	blocksPerRetarget := chaincfg.ShardEpochLength
	if config.ChainCtx.IsBeacon() {
		blocksPerRetarget = chaincfg.BeaconEpochLength
	}

	targetTimespan := int64(params.PowParams.TargetTimespan / time.Second)
	adjustmentFactor := params.PowParams.RetargetAdjustmentFactor
	b := BlockChain{
		db:       config.DB,
		chain:    config.ChainCtx,
		blockGen: config.BlockGen,
		// ------ ------ ------ ------ ------
		TimeSource:          config.TimeSource,
		SigCache:            config.SigCache,
		HashCache:           config.HashCache,
		indexManager:        config.IndexManager,
		checkpoints:         config.Checkpoints,
		checkpointsByHeight: checkpointsByHeight,

		retargetOpts: retargetOpts{
			minRetargetTimespan: targetTimespan / adjustmentFactor,
			maxRetargetTimespan: targetTimespan * adjustmentFactor,
			blocksPerRetarget:   int32(blocksPerRetarget),
		},
		blocksDB:         newBlocksStorage(config.DB, params),
		warningCaches:    newThresholdCaches(vbNumBits),
		deploymentCaches: newThresholdCaches(chaincfg.DefinedDeployments),
		dbFullRescan:     config.DBFullRescan,
		tryToRepairState: config.TryToRepairState,
	}

	// Initialize the chain state from the passed database.  When the db
	// does not yet contain any chain state, both it and the chain state
	// will be initialized to contain only the genesis block.
	if err := b.initChainState(); err != nil {
		return nil, err
	}

	// Perform any upgrades to the various chain-specific buckets as needed.
	if err := b.maybeUpgradeDBBuckets(config.Interrupt); err != nil {
		return nil, err
	}

	// Initialize and catch up all of the currently active optional indexes
	// as needed.
	if config.IndexManager != nil {
		err := config.IndexManager.Init(&b, config.Interrupt)
		if err != nil {
			return nil, err
		}
	}

	// Initialize rule change threshold state caches.
	if err := b.initThresholdCaches(); err != nil {
		return nil, err
	}

	bestNode := b.blocksDB.bestChain.Tip()
	log.Info().Str("chain", b.chain.Name()).Msgf("Chain state (height %d, hash %v, totaltx %d, work %v)",
		bestNode.Height(), bestNode.GetHash(), b.stateSnapshot.TotalTxns,
		bestNode.WorkSum())
	log.Info().Msgf("THE %s CHAIN IS READY", b.chain.Name())

	return &b, nil
}

// BlockChain provides functions for working with the bitcoin block chain.
// It includes functionality such as rejecting duplicate blocks, ensuring blocks
// follow all rules, orphan handling, checkpoint handling, and best chain
// selection with reorganization.
type BlockChain struct {
	// The following fields are set when the instance is created and can't
	// be changed afterwards, so there is no need to protect them with a
	// separate mutex.
	checkpoints         []chaincfg.Checkpoint
	checkpointsByHeight map[int32]*chaincfg.Checkpoint

	indexManager IndexManager

	// todo: combine this fields
	chain    chainctx.IChainCtx
	blockGen chaindata.ChainBlockGenerator
	db       database.DB
	// ------ ------ ------ ------ ------

	TimeSource chaindata.MedianTimeSource
	SigCache   *txscript.SigCache
	HashCache  *txscript.HashCache

	// The following fields are calculated based upon the provided chain
	// parameters.  They are also set when the instance is created and
	// can't be changed afterwards, so there is no need to protect them with
	retargetOpts retargetOpts

	// chainLock protects concurrent access to the vast majority of the
	// fields in this struct below this point.
	chainLock sync.RWMutex

	blocksDB rBlockStorage
	// These fields are related to checkpoint handling.  They are protected
	// by the chain lock.
	nextCheckpoint *chaincfg.Checkpoint
	checkpointNode blocknodes.IBlockNode

	// The state is used as a fairly efficient way to cache information
	// about the current best chain state that is returned to callers when
	// requested.  It operates on the principle of MVCC such that any time a
	// new block becomes the best block, the state pointer is replaced with
	// a new struct and the old state is left untouched.  In this way,
	// multiple callers can be pointing to different best chain states.
	// This is acceptable for most callers because the state is only being
	// queried at a specific point in time.
	//
	// In addition, some of the fields are stored in the database so the
	// chain state can be quickly reconstructed on load.
	stateLock     sync.RWMutex
	stateSnapshot *chaindata.BestState

	// The following caches are used to efficiently keep track of the
	// current deployment threshold state of each rule change deployment.
	//
	// This information is stored in the database so it can be quickly
	// reconstructed on load.
	//
	// warningCaches caches the current deployment threshold state for blocks
	// in each of the **possible** deployments.  This is used in order to
	// detect when new unrecognized rule changes are being voted on and/or
	// have been activated such as will be the case when older versions of
	// the software are being used
	//
	// deploymentCaches caches the current deployment threshold state for
	// blocks in each of the actively defined deployments.
	warningCaches    []thresholdStateCache
	deploymentCaches []thresholdStateCache

	// The following fields are used to determine if certain warnings have
	// already been shown.
	//
	// unknownRulesWarned refers to warnings due to unknown rules being
	// activated.
	//
	// unknownVersionsWarned refers to warnings due to unknown versions
	// being mined.
	unknownRulesWarned    bool
	unknownVersionsWarned bool

	// The notifications field stores a slice of callbacks to be executed on
	// certain blockchain events.
	notificationsLock sync.RWMutex
	notifications     []NotificationCallback

	// incidates if we are rescanning the whole db. Needed to enable fast catch-up, with processing only best chain
	dbFullRescan     bool
	tryToRepairState bool
}

// HaveBlock returns whether or not the chain instance has the block represented
// by the passed hash.  This includes checking the various places a block can
// be like part of the main chain, on a side chain, or in the orphan pool.
//
// This function is safe for concurrent access.
func (b *BlockChain) HaveBlock(hash *chainhash.Hash) (bool, error) {
	exists, err := b.blocksDB.blockExists(b.db, hash)
	if err != nil {
		return false, err
	}

	return exists || b.IsKnownOrphan(hash), nil
}

// IsKnownOrphan returns whether the passed hash is currently a known orphan.
// Keep in mind that only a limited number of orphans are held onto for a
// limited amount of time, so this function must not be used as an absolute
// way to test if a block is an orphan block.  A full block (as opposed to just
// its hash) must be passed to ProcessBlock for that purpose.  However, calling
// ProcessBlock with an orphan that already exists results in an error, so this
// function provides a mechanism for a caller to intelligently detect *recent*
// duplicate orphans and react accordingly.
//
// This function is safe for concurrent access.
func (b *BlockChain) IsKnownOrphan(hash *chainhash.Hash) bool {
	return b.blocksDB.orphanIndex.IsKnownOrphan(hash)
}

func (b *BlockChain) Chain() chainctx.IChainCtx {
	return b.chain
}

func (b *BlockChain) MMRTree() *mmr.BlocksMMRTree {
	return b.blocksDB.bestChain.mmrTree.BlocksMMRTree
}

func (b *BlockChain) ChainBlockGenerator() chaindata.ChainBlockGenerator {
	return b.blockGen
}

// CalcSequenceLock computes a relative lock-time SequenceLock for the passed
// transaction using the passed UtxoViewpoint to obtain the past median time
// for blocks in which the referenced inputs of the transactions were included
// within. The generated SequenceLock lock can be used in conjunction with a
// block height, and adjusted median block time to determine if all the inputs
// referenced within a transaction have reached sufficient maturity allowing
// the candidate transaction to be included in a block.
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcSequenceLock(tx *jaxutil.Tx, utxoView *chaindata.UtxoViewpoint,
	mempool bool) (*chaindata.SequenceLock, error) {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	return b.calcSequenceLock(b.blocksDB.bestChain.Tip(), tx, utxoView, mempool)
}

// calcSequenceLock computes the relative lock-times for the passed
// transaction. See the exported version, CalcSequenceLock for further details.
//
// This function MUST be called with the chain state lock held (for writes).
// nolint: gomnd
func (b *BlockChain) calcSequenceLock(node blocknodes.IBlockNode, tx *jaxutil.Tx,
	utxoView *chaindata.UtxoViewpoint, mempool bool) (*chaindata.SequenceLock, error) {
	// A value of -1 for each relative lock type represents a relative time
	// lock value that will allow a transaction to be included in a block
	// at any given height or time. This value is returned as the relative
	// lock time in the case that BIP 68 is disabled, or has not yet been
	// activated.
	sequenceLock := &chaindata.SequenceLock{Seconds: -1, BlockHeight: -1}

	// The sequence locks semantics are always active for transactions
	// within the mempool.
	csvSoftforkActive := mempool

	// If we're performing block validation, then we need to query the BIP9
	// state.
	// if !csvSoftforkActive {
	// 	// Obtain the latest BIP9 version bits state for the
	// 	// CSV-package soft-fork deployment. The adherence of sequence
	// 	// locks depends on the current soft-fork state.
	// 	csvState, err := b.deploymentState(node.Parent(), chaincfg.DeploymentCSV)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	csvSoftforkActive = csvState == ThresholdActive
	// }

	// If the transaction's version is less than 2, and BIP 68 has not yet
	// been activated then sequence locks are disabled. Additionally,
	// sequence locks don't apply to coinbase transactions Therefore, we
	// return sequence lock values of -1 indicating that this transaction
	// can be included within a block at any given height or time.
	mTx := tx.MsgTx()

	sequenceLockActive := mTx.Version == wire.TxVerTimeLock && csvSoftforkActive
	if !sequenceLockActive || chaindata.IsCoinBase(tx) {
		return sequenceLock, nil
	}

	// Grab the next height from the PoV of the passed blockNode to use for
	// inputs present in the mempool.
	nextHeight := node.Height() + 1

	for txInIndex, txIn := range mTx.TxIn {
		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)

		// ignore empty UTXO because this is normal situation for the SwapTx
		if utxo == nil && mTx.SwapTx() {
			// todo(mike): need to add more validation
			continue
		}

		if utxo == nil {
			str := fmt.Sprintf(
				"output %v referenced from transaction %s:%d either does not exist or has already been spent",
				txIn.PreviousOutPoint,
				tx.Hash(), txInIndex)
			return sequenceLock, chaindata.NewRuleError(chaindata.ErrMissingTxOut, str)
		}

		// If the input height is set to the mempool height, then we
		// assume the transaction makes it into the next block when
		// evaluating its sequence blocks.
		inputHeight := utxo.BlockHeight()
		if inputHeight == 0x7fffffff {
			inputHeight = nextHeight
		}

		// Given a sequence number, we apply the relative time lock
		// mask in order to obtain the time lock delta required before
		// this input can be spent.
		sequenceNum := txIn.Sequence
		relativeLock := int64(sequenceNum & wire.SequenceLockTimeMask)

		switch {
		// Relative time locks are disabled for this input, so we can
		// skip any further calculation.
		case sequenceNum&wire.SequenceLockTimeDisabled == wire.SequenceLockTimeDisabled:
			continue
		case sequenceNum&wire.SequenceLockTimeIsSeconds == wire.SequenceLockTimeIsSeconds:
			// This input requires a relative time lock expressed
			// in seconds before it can be spent.  Therefore, we
			// need to query for the block prior to the one in
			// which this input was included within so we can
			// compute the past median time for the block prior to
			// the one which included this referenced output.
			prevInputHeight := inputHeight - 1
			if prevInputHeight < 0 {
				prevInputHeight = 0
			}
			blockNode := node.Ancestor(prevInputHeight)
			medianTime := blockNode.CalcPastMedianTime()

			// Time based relative time-locks as defined by BIP 68
			// have a time granularity of RelativeLockSeconds, so
			// we shift left by this amount to convert to the
			// proper relative time-lock. We also subtract one from
			// the relative lock to maintain the original lockTime
			// semantics.
			timeLockSeconds := (relativeLock << wire.SequenceLockTimeGranularity) - 1
			timeLock := medianTime.Unix() + timeLockSeconds
			if timeLock > sequenceLock.Seconds {
				sequenceLock.Seconds = timeLock
			}
		default:
			// The relative lock-time for this input is expressed
			// in blocks so we calculate the relative offset from
			// the input's height as its converted absolute
			// lock-time. We subtract one from the relative lock in
			// order to maintain the original lockTime semantics.
			blockHeight := inputHeight + int32(relativeLock-1)
			if blockHeight > sequenceLock.BlockHeight {
				sequenceLock.BlockHeight = blockHeight
			}
		}

		mTx.TxIn[txInIndex].Age = nextHeight - inputHeight
	}

	return sequenceLock, nil
}

// LockTimeToSequence converts the passed relative locktime to a sequence
// number in accordance to BIP-68.
// See: https://github.com/bitcoin/bips/blob/master/bip-0068.mediawiki
//   - (Compatibility)
func LockTimeToSequence(isSeconds bool, locktime uint32) uint32 {
	// If we're expressing the relative lock time in blocks, then the
	// corresponding sequence number is simply the desired input age.
	if !isSeconds {
		return locktime
	}

	// Set the 22nd bit which indicates the lock time is in seconds, then
	// shift the locktime over by 9 since the time granularity is in
	// 512-second intervals (2^9). This results in a max lock-time of
	// 33,553,920 seconds, or 1.1 years.
	return wire.SequenceLockTimeIsSeconds |
		locktime>>wire.SequenceLockTimeGranularity
}

// connectBlock handles connecting the passed node/block to the end of the main
// (best) chain.
//
// This passed utxo view must have all referenced txos the block spends marked
// as spent and all of the new txos the block creates added to it.  In addition,
// the passed stxos slice must be populated with all of the information for the
// spent txos.  This approach is used because the connection validation that
// must happen prior to calling this function requires the same details, so
// it would be inefficient to repeat it.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) connectBlock(node blocknodes.IBlockNode, block *jaxutil.Block,
	view *chaindata.UtxoViewpoint, stxos []chaindata.SpentTxOut) error {
	// Make sure it's extending the end of the best chain.
	prevHash := node.PrevHash()
	bestBlockHash := b.blocksDB.bestChain.Tip().GetHash()
	if !prevHash.IsEqual(&bestBlockHash) {
		return chaindata.AssertError("connectBlock must be called with a block that extends the main chain")
	}

	// Sanity check the correct number of stxos are provided.
	if len(stxos) != chaindata.CountSpentOutputs(block) {
		return chaindata.AssertError("connectBlock called with inconsistent " +
			"spent transaction out information")
	}

	// No warnings about unknown rules or versions until the chain is
	// current.
	if b.isCurrent() {
		// Warn if any unknown new rules are either about to activate or
		// have already been activated.
		if err := b.warnUnknownRuleActivations(node); err != nil {
			return err
		}

		// Warn if a high enough percentage of the last blocks have
		// unexpected versions.
		if err := b.warnUnknownVersions(node); err != nil {
			return err
		}
	}

	// Write any block status changes to DB before updating best state.
	err := b.blocksDB.index.flushToDB()
	if err != nil {
		return err
	}

	// Generate a new best state snapshot that will be used to update the
	// database and later memory if all database updates are successful.
	b.stateLock.RLock()
	curTotalTxns := b.stateSnapshot.TotalTxns
	b.stateLock.RUnlock()
	numTxns := uint64(len(block.MsgBlock().Transactions))
	blockSize := uint64(block.MsgBlock().SerializeSize())
	blockWeight := uint64(chaindata.GetBlockWeight(block))

	// This node is now the end of the best chain.
	b.blocksDB.bestChain.SetTip(node)

	state := chaindata.NewBestState(node,
		b.blocksDB.bestChain.mmrTree.CurrentRoot(),
		blockSize,
		blockWeight,
		b.blocksDB.bestChain.mmrTree.CurrenWeight(),
		numTxns,
		curTotalTxns+numTxns,
		node.CalcPastMedianTime(),
		b.blocksDB.lastSerialID,
	)

	// Atomically insert info into the database.
	err = b.db.Update(func(dbTx database.Tx) error {
		// Update best block state.
		err = chaindata.RepoTx(dbTx).PutBestState(state, node.WorkSum())

		// Add the block hash and height to the block index which tracks
		// the main chain.
		err = chaindata.RepoTx(dbTx).PutBlockIndex(block.Hash(), node.Height())
		if err != nil {
			return err
		}

		// Update the utxo set using the state of the utxo view.  This
		// entails removing all of the utxos spent and adding the new
		// ones created by the block.
		err = chaindata.RepoTx(dbTx).PutUtxoView(view)
		if err != nil {
			return err
		}

		// Update the ead addresses set using the state of the utxo view.
		err = chaindata.RepoTx(dbTx).PutEADAddresses(view.EADAddressesSet())
		if err != nil {
			return err
		}

		// Update the transaction spend journal by adding a record for
		// the block that contains all txos spent by it.
		err = chaindata.RepoTx(dbTx).PutSpendJournalEntry(block.Hash(), stxos)
		if err != nil {
			return err
		}

		// Allow the index manager to call each of the currently active
		// optional indexes with the block being connected so they can
		// update themselves accordingly.
		if b.indexManager != nil {
			err := b.indexManager.ConnectBlock(dbTx, block, prevHash, stxos)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		// TODO: HERE must be disconnect from best chain
		return err
	}

	// Prune fully spent entries and mark all entries in the view unmodified
	// now that the modifications have been committed to the database.
	view.Commit()

	// Update the state for the best block.  Notice how this replaces the
	// entire struct instead of updating the existing one.  This effectively
	// allows the old version to act as a snapshot which callers can use
	// freely without needing to hold a lock for the duration.  See the
	// comments on the state variable for more details.
	b.stateLock.Lock()
	b.stateSnapshot = state
	b.stateLock.Unlock()

	// Notify the caller that the block was connected to the main chain.
	// The caller would typically want to react with actions such as
	// updating wallets.
	b.chainLock.Unlock()
	b.sendNotification(NTBlockConnected, block)
	b.chainLock.Lock()

	return nil
}

// disconnectBlock handles disconnecting the passed node/block from the end of
// the main (best) chain.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) disconnectBlock(node blocknodes.IBlockNode, block *jaxutil.Block, view *chaindata.UtxoViewpoint,
	forkRoot blocknodes.IBlockNode) error {
	// Make sure the node being disconnected is the end of the best chain.
	h := node.GetHash()
	th := b.blocksDB.bestChain.Tip().GetHash()
	if !h.IsEqual(&th) {
		return chaindata.AssertError("disconnectBlock must be called with the " +
			"block at the end of the main chain")
	}

	// Load the previous block since some details for it are needed below.
	prevNode := node.Parent()
	var prevBlock *jaxutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		prevBlock, err = chaindata.RepoTx(dbTx).FetchBlockByNode(prevNode)
		return err
	})
	if err != nil {
		return err
	}

	// Write any block status changes to DB before updating best state.
	err = b.blocksDB.index.flushToDB()
	if err != nil {
		return err
	}

	b.blocksDB.index.mmrTree.RmBlock(node.GetHash(), node.Height())
	// This node's parent is now the end of the best chain.
	b.blocksDB.bestChain.SetTip(node.Parent())

	// Generate a new best state snapshot that will be used to update the
	// database and later memory if all database updates are successful.
	b.stateLock.RLock()
	curTotalTxns := b.stateSnapshot.TotalTxns
	b.stateLock.RUnlock()
	numTxns := uint64(len(prevBlock.MsgBlock().Transactions))
	blockSize := uint64(prevBlock.MsgBlock().SerializeSize())
	blockWeight := uint64(chaindata.GetBlockWeight(prevBlock))
	newTotalTxns := curTotalTxns - uint64(len(block.MsgBlock().Transactions))

	state := chaindata.NewBestState(prevNode,
		node.PrevMMRRoot(),
		blockSize,
		blockWeight,
		b.blocksDB.bestChain.mmrTree.CurrenWeight(), // todo: review this
		numTxns,
		newTotalTxns,
		prevNode.CalcPastMedianTime(),
		b.blocksDB.lastSerialID,
	)

	err = b.db.Update(func(dbTx database.Tx) error {
		// get block serial id

		// Update best block state.
		err = chaindata.RepoTx(dbTx).PutBestState(state, node.WorkSum())
		if err != nil {
			return err
		}

		// Remove the block hash and height from the block index which
		// tracks the main chain.
		err = chaindata.RepoTx(dbTx).RemoveBlockIndex(block.Hash(), node.Height())
		if err != nil {
			return err
		}

		// Update the utxo set using the state of the utxo view.  This
		// entails restoring all of the utxos spent and removing the new
		// ones created by the block.
		err = chaindata.RepoTx(dbTx).PutUtxoView(view)
		if err != nil {
			return err
		}

		// Update the ead addresses set using the state of the utxo view.
		err = chaindata.RepoTx(dbTx).PutEADAddresses(view.EADAddressesSet())
		if err != nil {
			return err
		}

		// Before we delete the spend journal entry for this back,
		// we'll fetch it as is so the indexers can utilize if needed.
		stxos, err := chaindata.RepoTx(dbTx).FetchSpendJournalEntry(block)
		if err != nil {
			return err
		}

		// Update the transaction spend journal by removing the record
		// that contains all txos spent by the block.
		err = chaindata.RepoTx(dbTx).RemoveSpendJournalEntry(block.Hash())
		if err != nil {
			return err
		}

		// Allow the index manager to call each of the currently active
		// optional indexes with the block being disconnected so they
		// can update themselves accordingly.
		if b.indexManager != nil {
			err := b.indexManager.DisconnectBlock(dbTx, block, node.PrevHash(), stxos)
			if err != nil {
				return err
			}
		}

		if forkRoot != nil {
			return chaindata.RepoTx(dbTx).PutHashToSerialIDWithPrev(node.GetHash(),
				node.SerialID(), forkRoot.SerialID())
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Prune fully spent entries and mark all entries in the view unmodified
	// now that the modifications have been committed to the database.
	view.Commit()

	// Update the state for the best block.  Notice how this replaces the
	// entire struct instead of updating the existing one.  This effectively
	// allows the old version to act as a snapshot which callers can use
	// freely without needing to hold a lock for the duration.  See the
	// comments on the state variable for more details.
	b.stateLock.Lock()
	b.stateSnapshot = state
	b.stateLock.Unlock()

	// Notify the caller that the block was disconnected from the main
	// chain.  The caller would typically want to react with actions such as
	// updating wallets.
	b.chainLock.Unlock()
	b.sendNotification(NTBlockDisconnected, block)
	b.chainLock.Lock()

	return nil
}

// reorganizeChain reorganizes the block chain by disconnecting the nodes in the
// detachNodes list and connecting the nodes in the attach list.  It expects
// that the lists are already in the correct order and are in sync with the
// end of the current best chain.  Specifically, nodes that are being
// disconnected must be in reverse order (think of popping them off the end of
// the chain) and nodes the are being attached must be in forwards order
// (think pushing them onto the end of the chain).
//
// This function may modify node statuses in the block index without flushing.
//
// This function MUST be called with the chain state lock held (for writes).
// nolint: forcetypeassert
func (b *BlockChain) reorganizeChain(detachNodes, attachNodes *list.List) error {
	// Nothing to do if no reorganize nodes were provided.
	if detachNodes.Len() == 0 && attachNodes.Len() == 0 {
		return nil
	}

	var forkRoot blocknodes.IBlockNode

	// Ensure the provided nodes match the current best chain.
	tip := b.blocksDB.bestChain.Tip()
	if detachNodes.Len() != 0 {
		lastDetachNode := detachNodes.Back().Value.(blocknodes.IBlockNode)

		forkRoot = lastDetachNode.Parent()

		firstDetachNode := detachNodes.Front().Value.(blocknodes.IBlockNode)
		if firstDetachNode.GetHash() != tip.GetHash() {
			return chaindata.AssertError(fmt.Sprintf("reorganize nodes to detach are "+
				"not for the current best chain -- first detach node %v, "+
				"current chain %v", firstDetachNode.GetHash(), tip.GetHash()))
		}
	}

	// Ensure the provided nodes are for the same fork point.
	if attachNodes.Len() != 0 && detachNodes.Len() != 0 {
		firstAttachNode := attachNodes.Front().Value.(blocknodes.IBlockNode)
		lastDetachNode := detachNodes.Back().Value.(blocknodes.IBlockNode)
		if firstAttachNode.Parent().GetHash() != lastDetachNode.Parent().GetHash() {
			return chaindata.AssertError(fmt.Sprintf("reorganize nodes do not have the "+
				"same fork point -- first attach parent %v, last detach "+
				"parent %v", firstAttachNode.Parent().GetHash(),
				lastDetachNode.Parent().GetHash()))
		}
	}

	// Track the old and new best chains heads.
	oldBest := tip
	newBest := tip

	// All of the blocks to detach and related spend journal entries needed
	// to unspend transaction outputs in the blocks being disconnected must
	// be loaded from the database during the reorg check phase below and
	// then they are needed again when doing the actual database updates.
	// Rather than doing two loads, cache the loaded data into these slices.
	detachBlocks := make([]*jaxutil.Block, 0, detachNodes.Len())
	detachSpentTxOuts := make([][]chaindata.SpentTxOut, 0, detachNodes.Len())
	attachBlocks := make([]*jaxutil.Block, 0, attachNodes.Len())

	// Disconnect all of the blocks back to the point of the fork.  This
	// entails loading the blocks and their associated spent txos from the
	// database and using that information to unspend all of the spent txos
	// and remove the utxos created by the blocks.
	view := chaindata.NewUtxoViewpoint(b.chain.IsBeacon())
	h := oldBest.GetHash()
	view.SetBestHash(&h)
	for e := detachNodes.Front(); e != nil; e = e.Next() {
		blockNode := e.Value.(blocknodes.IBlockNode)
		var block *jaxutil.Block
		err := b.db.View(func(dbTx database.Tx) error {
			var err error
			block, err = chaindata.RepoTx(dbTx).FetchBlockByNode(blockNode)
			return err
		})
		if err != nil {
			return err
		}
		if blockNode.GetHash() != *block.Hash() {
			return chaindata.AssertError(fmt.Sprintf("detach block node hash %v (height "+
				"%v) does not match previous parent block hash %v", blockNode.GetHash(),
				blockNode.Height(), block.Hash()))
		}

		// Load all of the utxos referenced by the block that aren't
		// already in the view.
		err = view.FetchInputUtxos(b.db, block)
		if err != nil {
			return err
		}

		// Load all of the spent txos for the block from the spend
		// journal.
		var stxos []chaindata.SpentTxOut
		err = b.db.View(func(dbTx database.Tx) error {
			stxos, err = chaindata.RepoTx(dbTx).FetchSpendJournalEntry(block)
			return err
		})
		if err != nil {
			return err
		}

		// Store the loaded block and spend journal entry for later.
		detachBlocks = append(detachBlocks, block)
		detachSpentTxOuts = append(detachSpentTxOuts, stxos)
		err = view.DisconnectTransactions(b.db, block, blockNode.PrevHash(), stxos)
		if err != nil {
			return err
		}

		newBest = blockNode.Parent()
	}

	// Set the fork point only if there are nodes to attach since otherwise
	// blocks are only being disconnected and thus there is no fork point.
	var forkNode blocknodes.IBlockNode
	if attachNodes.Len() > 0 {
		forkNode = newBest
	}

	// Perform several checks to verify each block that needs to be attached
	// to the main chain can be connected without violating any rules and
	// without actually connecting the block.
	//
	// NOTE: These checks could be done directly when connecting a block,
	// however the downside to that approach is that if any of these checks
	// fail after disconnecting some blocks or attaching others, all of the
	// operations have to be rolled back to get the chain back into the
	// state it was before the rule violation (or other failure).  There are
	// at least a couple of ways accomplish that rollback, but both involve
	// tweaking the chain and/or database.  This approach catches these
	// issues before ever modifying the chain.
	for e := attachNodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(blocknodes.IBlockNode)

		var block *jaxutil.Block
		err := b.db.View(func(dbTx database.Tx) error {
			var err error
			block, err = chaindata.RepoTx(dbTx).FetchBlockByNode(n)
			return err
		})
		if err != nil {
			return err
		}

		// Store the loaded block for later.
		attachBlocks = append(attachBlocks, block)

		// Skip checks if node has already been fully validated. Although
		// checkConnectBlock gets skipped, we still need to update the UTXO
		// view.
		if b.blocksDB.index.NodeStatus(n).KnownValid() {
			err = view.FetchInputUtxos(b.db, block)
			if err != nil {
				return err
			}
			err = view.ConnectTransactions(block, nil)
			if err != nil {
				return err
			}

			newBest = n
			continue
		}

		// Notice the spent txout details are not requested here and
		// thus will not be generated.  This is done because the state
		// is not being immediately written to the database, so it is
		// not needed.
		//
		// In the case the block is determined to be invalid due to a
		// rule violation, mark it as invalid and mark all of its
		// descendants as having an invalid ancestor.
		err = b.checkConnectBlock(n, block, view, nil)
		if err != nil {
			if _, ok := err.(chaindata.RuleError); ok {
				b.blocksDB.index.SetStatusFlags(n, blocknodes.StatusValidateFailed)
				for de := e.Next(); de != nil; de = de.Next() {
					dn := de.Value.(blocknodes.IBlockNode)
					b.blocksDB.index.SetStatusFlags(dn, blocknodes.StatusInvalidAncestor)
				}
			}
			return err
		}
		b.blocksDB.index.SetStatusFlags(n, blocknodes.StatusValid)

		newBest = n
	}

	// Reset the view for the actual connection code below.  This is
	// required because the view was previously modified when checking if
	// the reorg would be successful and the connection code requires the
	// view to be valid from the viewpoint of each block being connected or
	// disconnected.
	view = chaindata.NewUtxoViewpoint(b.chain.IsBeacon())
	h = b.blocksDB.bestChain.Tip().GetHash()
	view.SetBestHash(&h)

	// Disconnect blocks from the main chain.
	for i, e := 0, detachNodes.Front(); e != nil; i, e = i+1, e.Next() {
		blockNode := e.Value.(blocknodes.IBlockNode)
		block := detachBlocks[i]

		// Load all of the utxos referenced by the block that aren't
		// already in the view.
		err := view.FetchInputUtxos(b.db, block)
		if err != nil {
			return err
		}

		// Update the view to unspend all of the spent txos and remove
		// the utxos created by the block.
		err = view.DisconnectTransactions(b.db, block, blockNode.PrevHash(), detachSpentTxOuts[i])
		if err != nil {
			return err
		}

		// Update the database and chain state.
		err = b.disconnectBlock(blockNode, block, view, forkRoot)
		if err != nil {
			return err
		}
	}

	// Connect the new best chain blocks.
	for i, e := 0, attachNodes.Front(); e != nil; i, e = i+1, e.Next() {
		n := e.Value.(blocknodes.IBlockNode)
		block := attachBlocks[i]

		// Load all of the utxos referenced by the block that aren't
		// already in the view.
		err := view.FetchInputUtxos(b.db, block)
		if err != nil {
			return err
		}

		// Update the view to mark all utxos referenced by the block
		// as spent and add all transactions being created by this block
		// to it.  Also, provide an stxo slice so the spent txout
		// details are generated.
		stxos := make([]chaindata.SpentTxOut, 0, chaindata.CountSpentOutputs(block))
		err = view.ConnectTransactions(block, &stxos)
		if err != nil {
			return err
		}

		// Update the database and chain state.
		err = b.connectBlock(n, block, view, stxos)
		if err != nil {
			return err
		}
	}

	// Log the point where the chain forked and old and new best chain
	// heads.
	if forkNode != nil {
		log.Info().Str("chain", b.chain.Name()).Msgf("REORGANIZE: ChainCtx forks at %v (height %v)", forkNode.GetHash(),
			forkNode.Height())
	}
	log.Info().Str("chain", b.chain.Name()).Msgf("REORGANIZE: Old best chain head was %v (height %v)",
		oldBest.GetHash(), oldBest.Height())
	log.Info().Str("chain", b.chain.Name()).Msgf("REORGANIZE: New best chain head is %v (height %v)",
		newBest.GetHash(), newBest.Height())

	return nil
}

// connectBestChain handles connecting the passed block to the chain while
// respecting proper chain selection according to the chain with the most
// proof of work.  In the typical case, the new block simply extends the main
// chain.  However, it may also be extending (or creating) a side chain (fork)
// which may or may not end up becoming the main chain depending on which fork
// cumulatively has the most proof of work.  It returns whether or not the block
// ended up on the main chain (either due to extending the main chain or causing
// a reorganization to become the main chain).
//
// The flags modify the behavior of this function as follows:
//   - BFFastAdd: Avoids several expensive transaction validation operations.
//     This is useful when using checkpoints.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) connectBestChain(node blocknodes.IBlockNode, block *jaxutil.Block, flags chaindata.BehaviorFlags) (bool, error) {
	fastAdd := flags&chaindata.BFFastAdd == chaindata.BFFastAdd

	flushIndexState := func() {
		// Intentionally ignore errors writing updated node status to DB. If
		// it fails to write, it's not the end of the world. If the block is
		// valid, we flush in connectBlock and if the block is invalid, the
		// worst that can happen is we revalidate the block after a restart.
		if writeErr := b.blocksDB.index.flushToDB(); writeErr != nil {
			log.Warn().Str("chain", b.chain.Name()).Msgf("Error flushing block index changes to disk: %v",
				writeErr)
		}
	}

	// We are extending the main (best) chain with a new block.  This is the
	// most common case.
	parentHash := node.PrevHash()
	bestBlockHash := b.blocksDB.bestChain.Tip().GetHash()
	if parentHash.IsEqual(&bestBlockHash) {
		// Skip checks if node has already been fully validated.
		fastAdd = fastAdd || b.blocksDB.index.NodeStatus(node).KnownValid()

		// Perform several checks to verify the block can be connected
		// to the main chain without violating any rules and without
		// actually connecting the block.
		view := chaindata.NewUtxoViewpoint(b.chain.IsBeacon())
		view.SetBestHash(&parentHash)
		stxos := make([]chaindata.SpentTxOut, 0, chaindata.CountSpentOutputs(block))
		if !fastAdd {
			err := b.checkConnectBlock(node, block, view, &stxos)
			if err == nil {
				b.blocksDB.index.SetStatusFlags(node, blocknodes.StatusValid)
			} else if _, ok := err.(chaindata.RuleError); ok {
				b.blocksDB.index.SetStatusFlags(node, blocknodes.StatusValidateFailed)
			} else {
				return false, err
			}

			flushIndexState()

			if err != nil {
				return false, err
			}
		}

		// In the fast add case the code to check the block connection
		// was skipped, so the utxo view needs to load the referenced
		// utxos, spend them, and add the new utxos being created by
		// this block.
		if fastAdd {
			err := view.FetchInputUtxos(b.db, block)
			if err != nil {
				return false, err
			}
			err = view.ConnectTransactions(block, &stxos)
			if err != nil {
				return false, err
			}
		}

		// Connect the block to the main chain.
		err := b.connectBlock(node, block, view, stxos)
		if err != nil {
			// If we got hit with a rule error, then we'll mark
			// that status of the block as invalid and flush the
			// index state to disk before returning with the error.
			if _, ok := err.(chaindata.RuleError); ok {
				b.blocksDB.index.SetStatusFlags(node, blocknodes.StatusValidateFailed)
			}

			flushIndexState()

			return false, err
		}

		// If this is fast add, or this block node isn't yet marked as
		// valid, then we'll update its status and flush the state to
		// disk again.
		if fastAdd || !b.blocksDB.index.NodeStatus(node).KnownValid() {
			b.blocksDB.index.SetStatusFlags(node, blocknodes.StatusValid)
			flushIndexState()
		}

		return true, nil
	}
	if fastAdd {
		log.Warn().Str("chain", b.chain.Name()).Msgf("fastAdd set in the side chain case? %v\n", block.Hash())
	}

	// We're extending (or creating) a side chain, but the cumulative
	// work for this new side chain is not enough to make it the new chain.
	if node.WorkSum().Cmp(b.blocksDB.bestChain.Tip().WorkSum()) <= 0 {
		// Log information about how the block is forking the chain.
		fork := b.blocksDB.bestChain.FindFork(node)
		if fork.GetHash() == parentHash {
			log.Info().Str("chain", b.chain.Name()).Msgf("FORK: Block %v forks the chain at height %d"+
				"/block %v, but does not cause a reorganize",
				node.GetHash(), fork.Height(), fork.GetHash())
		} else {
			log.Info().Str("chain", b.chain.Name()).Msgf("EXTEND FORK: Block %v extends a side chain "+
				"which forks the chain at height %d/block %v",
				node.GetHash(), fork.Height(), fork.GetHash())
		}

		return false, nil
	}

	// We're extending (or creating) a side chain and the cumulative work
	// for this new side chain is more than the old best chain, so this side
	// chain needs to become the main chain.  In order to accomplish that,
	// find the common ancestor of both sides of the fork, disconnect the
	// blocks that form the (now) old fork from the main chain, and attach
	// the blocks that form the new chain to the main chain starting at the
	// common ancenstor (the point where the chain forked).
	detachNodes, attachNodes := b.blocksDB.getReorganizeNodes(node)

	// Reorganize the chain.
	log.Info().Str("chain", b.chain.Name()).Msgf("REORGANIZE: Block %v is causing a reorganize.", node.GetHash())
	err := b.reorganizeChain(detachNodes, attachNodes)

	// Either getReorganizeNodes or reorganizeChain could have made unsaved
	// changes to the block index, so flush regardless of whether there was an
	// error. The index would only be dirty if the block failed to connect, so
	// we can ignore any errors writing.
	if writeErr := b.blocksDB.index.flushToDB(); writeErr != nil {
		log.Warn().Str("chain", b.chain.Name()).Msgf("Error flushing block index changes to disk: %v", writeErr)
	}

	return err == nil, err
}

// isCurrent returns whether or not the chain believes it is current.  Several
// factors are used to guess, but the key factors that allow the chain to
// believe it is current are:
//   - Latest block height is after the latest checkpoint (if enabled)
//   - Latest block has a timestamp newer than 24 hours ago
//
// This function MUST be called with the chain state lock held (for reads).
// For a development environment (chaincfg.FastNetParams),
// we ignore that the timestamp of the highest block is out of date.
func (b *BlockChain) isCurrent() bool {
	// Not current if the latest main (best) chain height is before the
	// latest known good checkpoint (when checkpoints are enabled).
	checkpoint := b.LatestCheckpoint()
	if checkpoint != nil && b.blocksDB.bestChain.Tip().Height() < checkpoint.Height {
		return false
	}

	chainName := b.chain.Params().Net
	developmentEnv := chainName == wire.FastTestNet || chainName == wire.SimNet || chainName == wire.TestNet
	if developmentEnv {
		return true
	}

	// Not current if the latest best block has a timestamp before 24 hours
	// ago.
	//
	// The chain appears to be current if none of the checks reported
	// otherwise.
	// minus24Hours := b.TimeSource.AdjustedTime().Add(-24 * time.Hour).Unix()
	minus1Week := b.TimeSource.AdjustedTime().Add(-1 * 7 * 24 * time.Hour).Unix()
	// todo: rollback this when network become more stable
	return b.blocksDB.bestChain.Tip().Timestamp() >= minus1Week
}

// IsCurrent returns whether or not the chain believes it is current.  Several
// factors are used to guess, but the key factors that allow the chain to
// believe it is current are:
//   - Latest block height is after the latest checkpoint (if enabled)
//   - Latest block has a timestamp newer than 24 hours ago
//
// This function is safe for concurrent access.
func (b *BlockChain) IsCurrent() bool {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	return b.isCurrent()
}

// BestSnapshot returns information about the current best chain block and
// related state as of the current point in time.  The returned instance must be
// treated as immutable since it is shared by all callers.
//
// This function is safe for concurrent access.
func (b *BlockChain) BestSnapshot() *chaindata.BestState {
	b.stateLock.RLock()
	snapshot := b.stateSnapshot
	b.stateLock.RUnlock()
	return snapshot
}

// ShardCount returns the actual number of shards in network,
// based on information form latest beacon header.
//
// This function is safe for concurrent access.
func (b *BlockChain) ShardCount() (uint32, error) {
	b.stateLock.RLock()
	snapshot := b.stateSnapshot
	b.stateLock.RUnlock()

	return snapshot.Shards, nil
}

// HeaderByHash returns the block header identified by the given hash or an
// error if it doesn't exist. Note that this will return headers from both the
// main and side chains.
func (b *BlockChain) HeaderByHash(hash *chainhash.Hash) (wire.BlockHeader, error) {
	node := b.blocksDB.index.LookupNode(hash)
	if node == nil {
		err := fmt.Errorf("block %s is not known", hash)
		return b.chain.EmptyHeader(), err
	}

	return node.Header(), nil
}

// MainChainHasBlock returns whether or not the block with the given hash is in
// the main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) MainChainHasBlock(hash *chainhash.Hash) bool {
	node := b.blocksDB.index.LookupNode(hash)
	return node != nil && b.blocksDB.bestChain.Contains(node)
}

// BlockLocatorFromHash returns a block locator for the passed block hash.
// See BlockLocator for details on the algorithm used to create a block locator.
//
// In addition to the general algorithm referenced above, this function will
// return the block locator for the latest known tip of the main (best) chain if
// the passed hash is not currently known.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockLocatorFromHash(hash *chainhash.Hash) BlockLocator {
	b.chainLock.RLock()
	node := b.blocksDB.index.LookupNode(hash)
	locator := b.blocksDB.bestChain.blockLocator(node)
	b.chainLock.RUnlock()
	return locator
}

// LatestBlockLocator returns a block locator for the latest known tip of the
// main (best) chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) LatestBlockLocator() (BlockLocator, error) {
	b.chainLock.RLock()
	locator := b.blocksDB.bestChain.BlockLocator(nil)
	b.chainLock.RUnlock()
	return locator, nil
}

// BlockHeightByHash returns the height of the block with the given hash in the
// main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockHeightByHash(hash *chainhash.Hash) (int32, error) {
	node := b.blocksDB.index.LookupNode(hash)
	if node == nil || !b.blocksDB.bestChain.Contains(node) {
		str := fmt.Sprintf("block %s is not in the main chain", hash)
		return 0, chaindata.ErrNotInMainChain(str)
	}

	return node.Height(), nil
}

// BlockHashByHeight returns the hash of the block at the given height in the
// main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockHashByHeight(blockHeight int32) (*chainhash.Hash, error) {
	node := b.blocksDB.bestChain.NodeByHeight(blockHeight)
	if node == nil {
		str := fmt.Sprintf("no block at height %d exists", blockHeight)
		return nil, chaindata.ErrNotInMainChain(str)

	}

	h := node.GetHash()
	return &h, nil
}

// HeightRange returns a range of block hashes for the given start and end
// heights.  It is inclusive of the start height and exclusive of the end
// height.  The end height will be limited to the current main chain height.
//
// This function is safe for concurrent access.
func (b *BlockChain) HeightRange(startHeight, endHeight int32) ([]chainhash.Hash, error) {
	// Ensure requested heights are sane.
	if startHeight < 0 {
		return nil, fmt.Errorf("start height of fetch range must not "+
			"be less than zero - got %d", startHeight)
	}
	if endHeight < startHeight {
		return nil, fmt.Errorf("end height of fetch range must not "+
			"be less than the start height - got start %d, end %d",
			startHeight, endHeight)
	}

	// There is nothing to do when the start and end heights are the same,
	// so return now to avoid the chain view lock.
	if startHeight == endHeight {
		return nil, nil
	}

	// Grab a lock on the chain view to prevent it from changing due to a
	// reorg while building the hashes.
	b.blocksDB.bestChain.mtx.Lock()
	defer b.blocksDB.bestChain.mtx.Unlock()

	// When the requested start height is after the most recent best chain
	// height, there is nothing to do.
	latestHeight := b.blocksDB.bestChain.tip().Height()
	if startHeight > latestHeight {
		return nil, nil
	}

	// Limit the ending height to the latest height of the chain.
	if endHeight > latestHeight+1 {
		endHeight = latestHeight + 1
	}

	// Fetch as many as are available within the specified range.
	hashes := make([]chainhash.Hash, 0, endHeight-startHeight)
	for i := startHeight; i < endHeight; i++ {
		hashes = append(hashes, b.blocksDB.bestChain.nodeByHeight(i).GetHash())
	}
	return hashes, nil
}

// HeightToHashRange returns a range of block hashes for the given start height
// and end hash, inclusive on both ends.  The hashes are for all blocks that are
// ancestors of endHash with height greater than or equal to startHeight.  The
// end hash must belong to a block that is known to be valid.
//
// This function is safe for concurrent access.
func (b *BlockChain) HeightToHashRange(startHeight int32,
	endHash *chainhash.Hash, maxResults int) ([]chainhash.Hash, error) {
	endNode := b.blocksDB.index.LookupNode(endHash)
	if endNode == nil {
		return nil, fmt.Errorf("no known block header with hash %v", endHash)
	}
	if !b.blocksDB.index.NodeStatus(endNode).KnownValid() {
		return nil, fmt.Errorf("block %v is not yet validated", endHash)
	}
	endHeight := endNode.Height()

	if startHeight < 0 {
		return nil, fmt.Errorf("start height (%d) is below 0", startHeight)
	}
	if startHeight > endHeight {
		return nil, fmt.Errorf("start height (%d) is past end height (%d)",
			startHeight, endHeight)
	}

	resultsLength := int(endHeight - startHeight + 1)
	if resultsLength > maxResults {
		return nil, fmt.Errorf("number of results (%d) would exceed max (%d)",
			resultsLength, maxResults)
	}

	// Walk backwards from endHeight to startHeight, collecting block hashes.
	node := endNode
	hashes := make([]chainhash.Hash, resultsLength)
	for i := resultsLength - 1; i >= 0; i-- {
		hashes[i] = node.GetHash()
		node = node.Parent()
	}
	return hashes, nil
}

// IntervalBlockHashes returns hashes for all blocks that are ancestors of
// endHash where the block height is a positive multiple of interval.
//
// This function is safe for concurrent access.
func (b *BlockChain) IntervalBlockHashes(endHash *chainhash.Hash, interval int) ([]chainhash.Hash, error) {
	endNode := b.blocksDB.index.LookupNode(endHash)
	if endNode == nil {
		return nil, fmt.Errorf("no known block header with hash %v", endHash)
	}
	if !b.blocksDB.index.NodeStatus(endNode).KnownValid() {
		return nil, fmt.Errorf("block %v is not yet validated", endHash)
	}
	endHeight := endNode.Height()

	resultsLength := int(endHeight) / interval
	hashes := make([]chainhash.Hash, resultsLength)

	b.blocksDB.bestChain.mtx.Lock()
	defer b.blocksDB.bestChain.mtx.Unlock()

	blockNode := endNode
	for index := int(endHeight) / interval; index > 0; index-- {
		// Use the bestChain chainView for faster lookups once lookup intersects
		// the best chain.
		blockHeight := int32(index * interval)
		if b.blocksDB.bestChain.contains(blockNode) {
			blockNode = b.blocksDB.bestChain.nodeByHeight(blockHeight)
		} else {
			blockNode = blockNode.Ancestor(blockHeight)
		}

		hashes[index-1] = blockNode.GetHash()
	}

	return hashes, nil
}

// locateInventory returns the node of the block after the first known block in
// the locator along with the number of subsequent nodes needed to either reach
// the provided stop hash or the provided max number of entries.
//
// In addition, there are two special cases:
//
//   - When no locators are provided, the stop hash is treated as a request for
//     that block, so it will either return the node associated with the stop hash
//     if it is known, or nil if it is unknown
//   - When locators are provided, but none of them are known, nodes starting
//     after the genesis block will be returned
//
// This is primarily a helper function for the locateBlocks and locateHeaders
// functions.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) locateInventory(locator BlockLocator, hashStop *chainhash.Hash, maxEntries uint32) (blocknodes.IBlockNode, uint32) {
	// There are no block locators so a specific block is being requested
	// as identified by the stop hash.
	stopNode := b.blocksDB.index.LookupNode(hashStop)
	if len(locator) == 0 {
		if stopNode == nil {
			// No blocks with the stop hash were found so there is
			// nothing to do.
			return nil, 0
		}
		return stopNode, 1
	}

	// Find the most recent locator block hash in the main chain.  In the
	// case none of the hashes in the locator are in the main chain, fall
	// back to the genesis block.
	startNode := b.blocksDB.bestChain.Genesis()
	for _, blockMeta := range locator {
		node := b.blocksDB.index.LookupNode(&blockMeta.Hash)
		if node != nil && b.blocksDB.bestChain.Contains(node) {
			startNode = node
			break
		}
	}

	// Start at the block after the most recently known block.  When there
	// is no next block it means the most recently known block is the tip of
	// the best chain, so there is nothing more to do.
	startNode = b.blocksDB.bestChain.Next(startNode)
	if startNode == nil {
		return nil, 0
	}

	// Calculate how many entries are needed.
	total := uint32((b.blocksDB.bestChain.Tip().Height() - startNode.Height()) + 1)
	if stopNode != nil && b.blocksDB.bestChain.Contains(stopNode) &&
		stopNode.Height() >= startNode.Height() {

		total = uint32((stopNode.Height() - startNode.Height()) + 1)
	}
	if total > maxEntries {
		total = maxEntries
	}

	return startNode, total
}

// locateBlocks returns the hashes of the blocks after the first known block in
// the locator until the provided stop hash is reached, or up to the provided
// max number of block hashes.
//
// See the comment on the exported function for more details on special cases.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) locateBlocks(locator BlockLocator, hashStop *chainhash.Hash, maxHashes uint32) []chainhash.Hash {
	// Find the node after the first known block in the locator and the
	// total number of nodes after it needed while respecting the stop hash
	// and max entries.
	node, total := b.locateInventory(locator, hashStop, maxHashes)
	if total == 0 {
		return nil
	}

	// Populate and return the found hashes.
	hashes := make([]chainhash.Hash, 0, total)
	for i := uint32(0); i < total; i++ {
		hashes = append(hashes, node.GetHash())
		node = b.blocksDB.bestChain.Next(node)
	}
	return hashes
}

// LocateBlocks returns the hashes of the blocks after the first known block in
// the locator until the provided stop hash is reached, or up to the provided
// max number of block hashes.
//
// In addition, there are two special cases:
//
//   - When no locators are provided, the stop hash is treated as a request for
//     that block, so it will either return the stop hash itself if it is known,
//     or nil if it is unknown
//   - When locators are provided, but none of them are known, hashes starting
//     after the genesis block will be returned
//
// This function is safe for concurrent access.
func (b *BlockChain) LocateBlocks(locator BlockLocator, hashStop *chainhash.Hash, maxHashes uint32) []chainhash.Hash {
	b.chainLock.RLock()
	hashes := b.locateBlocks(locator, hashStop, maxHashes)
	b.chainLock.RUnlock()
	return hashes
}

// locateHeaders returns the headers of the blocks after the first known block
// in the locator until the provided stop hash is reached, or up to the provided
// max number of block headers.
//
// See the comment on the exported function for more details on special cases.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) locateHeaders(locator BlockLocator, hashStop *chainhash.Hash, maxHeaders uint32) []wire.HeaderBox {
	// Find the node after the first known block in the locator and the
	// total number of nodes after it needed while respecting the stop hash
	// and max entries.
	node, total := b.locateInventory(locator, hashStop, maxHeaders)
	if total == 0 {
		return nil
	}

	// Populate and return the found headers.
	headers := make([]wire.HeaderBox, 0, total)
	for i := uint32(0); i < total; i++ {
		headers = append(headers, wire.HeaderBox{Header: node.NewHeader(), ActualMMRRoot: node.ActualMMRRoot()})
		node = b.blocksDB.bestChain.Next(node)
	}
	return headers
}

// LocateHeaders returns the headers of the blocks after the first known block
// in the locator until the provided stop hash is reached, or up to a max of
// wire.MaxBlockHeadersPerMsg headers.
//
// In addition, there are two special cases:
//
//   - When no locators are provided, the stop hash is treated as a request for
//     that header, so it will either return the header for the stop hash itself
//     if it is known, or nil if it is unknown
//   - When locators are provided, but none of them are known, headers starting
//     after the genesis block will be returned
//
// This function is safe for concurrent access.
func (b *BlockChain) LocateHeaders(locator BlockLocator, hashStop *chainhash.Hash) []wire.HeaderBox {
	b.chainLock.RLock()
	headers := b.locateHeaders(locator, hashStop, wire.MaxBlockHeadersPerMsg)
	b.chainLock.RUnlock()
	return headers
}

// maybeUpgradeDBBuckets checks the database version of the buckets used by this
// package and performs any needed upgrades to bring them to the latest version.
//
// All buckets used by this package are guaranteed to be the latest version if
// this function returns without error.
func (b *BlockChain) maybeUpgradeDBBuckets(interrupt <-chan struct{}) error {
	// Load or create bucket versions as needed.
	var utxoSetVersion uint32
	err := b.db.Update(func(dbTx database.Tx) error {
		// Load the utxo set version from the database or create it and
		// initialize it to version 1 if it doesn't exist.
		var err error
		utxoSetVersion, err = chaindata.RepoTx(dbTx).FetchOrCreateVersion(
			chaindata.UtxoSetVersionKeyName, 1)
		return err
	})
	if err != nil {
		return err
	}

	// Update the utxo set to v2 if needed.
	if utxoSetVersion < 2 {
		if err := chaindata.UpgradeUtxoSetToV2(b.db, interrupt); err != nil {
			return err
		}
	}

	return nil
}

// BlockSerialIDByHash returns the serialID and previous serialID of the block with the given hash.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockSerialIDByHash(block *chainhash.Hash) (int64, int64, error) {
	var serialID, prevSerialID int64

	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		serialID, prevSerialID, err = chaindata.RepoTx(dbTx).FetchBlockSerialID(block)
		return err
	})

	return serialID, prevSerialID, err
}

// BlockIDsByHash returns the height, serialID and previous serialID of the block with the given hash.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockIDsByHash(hash *chainhash.Hash) (int32, int64, int64, error) {
	var height int32
	var serialID, prevSerialID int64
	var err error
	node := b.blocksDB.index.LookupNode(hash)
	if node == nil || !b.blocksDB.bestChain.Contains(node) {
		// block %s is not in the main chain
		height = -1

		err = b.db.View(func(dbTx database.Tx) error {
			var err error
			serialID, prevSerialID, err = chaindata.RepoTx(dbTx).FetchBlockSerialID(hash)
			return err
		})
		return height, serialID, prevSerialID, err
	}

	height = node.Height()
	serialID = node.SerialID()
	if node.Parent() != nil {
		prevSerialID = node.Parent().SerialID()
	}

	return height, serialID, prevSerialID, err
}

func (b *BlockChain) SaveBestChainSerialIDs() error {
	serialIDs := make([]int64, 0, len(b.blocksDB.bestChain.nodes))

	err := b.db.Update(func(dbTx database.Tx) error {
		for i := range b.blocksDB.bestChain.nodes {
			serialIDs = append(serialIDs, b.blocksDB.bestChain.nodes[i].SerialID())
			err := chaindata.RepoTx(dbTx).PutMMRRoot(
				b.blocksDB.bestChain.nodes[i].ActualMMRRoot(),
				b.blocksDB.bestChain.nodes[i].GetHash())
			if err != nil {
				return err
			}
		}

		return chaindata.RepoTx(dbTx).PutSerialIDsList(serialIDs)
	})
	if err != nil {
		return err
	}

	return nil
}
