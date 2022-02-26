// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// FetchSpendJournal attempts to retrieve the spend journal, or the set of
// outputs spent for the target block. This provides a view of all the outputs
// that will be consumed once the target block is connected to the end of the
// main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) FetchSpendJournal(targetBlock *jaxutil.Block) ([]chaindata.SpentTxOut, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	var spendEntries []chaindata.SpentTxOut
	err := b.db.View(func(dbTx database.Tx) error {
		var err error

		spendEntries, err = chaindata.DBFetchSpendJournalEntry(dbTx, targetBlock)
		return err
	})
	if err != nil {
		return nil, err
	}

	return spendEntries, nil
}

// BlockByHeight returns the block at the given height in the main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockByHeight(blockHeight int32) (*jaxutil.Block, error) {
	// Lookup the block height in the best chain.
	node := b.blocksDB.bestChain.NodeByHeight(blockHeight)
	if node == nil {
		str := fmt.Sprintf("no block at height %d exists", blockHeight)
		return nil, chaindata.ErrNotInMainChain(str)
	}

	// Load the block from the database and return it.
	var block *jaxutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		block, err = chaindata.DBFetchBlockByNode(dbTx, node)
		return err
	})
	return block, err
}

// BlockByHash returns the block from the main chain with the given hash with
// the appropriate chain height set.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockByHash(hash *chainhash.Hash) (*jaxutil.Block, error) {
	// Lookup the block hash in block index and ensure it is in the best
	// chain.
	node := b.blocksDB.index.LookupNode(hash)
	if node == nil || !b.blocksDB.bestChain.Contains(node) {
		str := fmt.Sprintf("block %s is not in the main chain", hash)
		return nil, chaindata.ErrNotInMainChain(str)
	}

	// Load the block from the database and return it.
	var block *jaxutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		block, err = chaindata.DBFetchBlockByNode(dbTx, node)
		return err
	})
	return block, err
}

// initChainState attempts to load and initialize the chain state from the
// database.  When the db does not yet contain any chain state, both it and the
// chain state are initialized to the genesis block.
// nolint: gocritic
func (b *BlockChain) initChainState() error {
	var err error
	b.stateSnapshot, err = initChainState(b.db, &b.blocksDB, b.dbFullRescan)
	if err != nil && !b.dbFullRescan && b.tryToRepairState {
		b.stateSnapshot, err = tryToLoadAndRepairState(b.db, &b.blocksDB)
	}
	return err
}

// createChainState initializes both the database and the chain state to the
// genesis block.  This includes creating the necessary buckets and inserting
// the genesis block, so it must only be called on an uninitialized database.
func (b *BlockChain) createChainState() error {
	var err error
	b.stateSnapshot, err = createChainState(b.db, &b.blocksDB)
	return err
}

func (b *BlockChain) fastInitChainState() (bool, error) {
	ok, best, err := fastInitChainState(b.db, &b.blocksDB)
	b.stateSnapshot = best
	return ok, err
}

// initChainState attempts to load and initialize the chain state from the
// database.  When the db does not yet contain any chain state, both it and the
// chain state are initialized to the genesis block.
// nolint: gocritic
func initChainState(db database.DB, blocksDB *rBlockStorage, dbFullRescan bool) (*chaindata.BestState, error) {
	// Determine the state of the chain database. We may need to initialize
	// everything from scratch or upgrade certain buckets.
	var (
		initialized             bool
		bestChainSnapshotExists bool
	)

	err := db.View(func(dbTx database.Tx) error {
		initialized = dbTx.Metadata().Get(chaindata.ChainStateKeyName) != nil
		bestChainSnapshotExists = dbTx.Metadata().Bucket(chaindata.BestChainSerialIDsBucketName) != nil
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !initialized {
		// At this point the database has not already been initialized, so
		// initialize both it and the chain state to the genesis block.
		return createChainState(db, blocksDB)
	}

	if bestChainSnapshotExists && !dbFullRescan {
		rescanRequired, bestState, err := fastInitChainState(db, blocksDB)
		if !rescanRequired || err != nil {
			return bestState, err
		}
	}

	var stateSnapshot *chaindata.BestState
	// Attempt to load the chain state from the database.
	err = db.View(func(dbTx database.Tx) error {
		// Fetch the stored chain state from the database metadata.
		// When it doesn't exist, it means the database hasn't been
		// initialized for use with chain yet, so break out now to allow
		// that to happen under a writable database transaction.
		serializedData := dbTx.Metadata().Get(chaindata.ChainStateKeyName)
		log.Trace().Str("chain", db.Chain().Name()).Msgf("Serialized chain state: %x", serializedData)
		state, err := chaindata.DeserializeBestChainState(serializedData)
		if err != nil {
			return err
		}

		// Load all of the headers from the data for the known best
		// chain and construct the block index accordingly.  Since the
		// number of nodes are already known, perform a single alloc
		// for them versus a bunch of little ones to reduce
		// pressure on the GC.
		log.Info().Str("chain", db.Chain().Name()).Msgf("Loading block index...")

		blockIndexBucket := dbTx.Metadata().Bucket(chaindata.BlockIndexBucketName)

		var i int32
		var lastNode blocknodes.IBlockNode
		cursor := blockIndexBucket.Cursor()

		for ok := cursor.First(); ok; ok = cursor.Next() {
			header, status, blockSerialID, err := chaindata.DeserializeBlockRow(cursor.Value())
			if err != nil {
				return err
			}

			// Determine the parent block node. Since we iterate block headers
			// in order of height, if the blocks are mostly linear there is a
			// very good chance the previous header processed is the parent.
			var parent blocknodes.IBlockNode

			if lastNode == nil {
				blockHash := header.BlockHash()

				if !blockHash.IsEqual(db.Chain().Params().GenesisHash()) {
					return chaindata.AssertError(fmt.Sprintf(
						"initChainState: Expected first entry in block index to be genesis block: expected %s, found %s",
						db.Chain().Params().GenesisHash(), blockHash))
				}
			} else if header.PrevBlocksMMRRoot() == blocksDB.index.MMRTreeRoot() {
				// } else if header.PrevBlockHash() == lastNode.GetHash() {
				// Since we iterate block headers in order of height, if the
				// blocks are mostly linear there is a very good chance the
				// previous header processed is the parent.
				parent = lastNode
			} else {
				prev := header.PrevBlocksMMRRoot()
				parent = blocksDB.index.LookupNodeByMMRRoot(prev)
				// hash := header.PrevBlockHash()
				// parent = blocksDB.index.LookupNode(&hash)
				if parent == nil {
					return chaindata.AssertError(fmt.Sprintf(
						"initChainState: Could not find parent for block %s", header.BlockHash()))
				}
			}

			if parent != nil {
				prevHash := parent.GetHash()
				bph := header.PrevBlockHash()
				if !prevHash.IsEqual(&bph) {
					str := fmt.Sprintf("hash(%s) of parent resolved by mmr(%s) not match with hash(%s) from header",
						prevHash, header.PrevBlocksMMRRoot(), bph)
					return chaindata.AssertError(str)
				}
			}

			// Initialize the block node for the block, connect it,
			// and add it to the block index.
			node := db.Chain().NewNode(header, parent, blockSerialID)
			node.SetStatus(status)

			blocksDB.index.addNode(node, false)

			lastNode = node
			i++

			if i%1000 == 0 {
				log.Info().Str("chain", db.Chain().Name()).
					Int32("processed_blocks_count", i).
					Msgf("Processing raw blocks...")
			}
		}

		blocksDB.lastSerialID = state.LastSerialID

		// Set the best chain view to the stored best state.
		tip := blocksDB.index.LookupNode(&state.Hash)
		if tip == nil {
			return chaindata.AssertError(fmt.Sprintf(
				"initChainState: cannot find chain tip %s in block index", state.Hash))
		}
		blocksDB.bestChain.SetTip(tip)

		// Load the raw block bytes for the best block.
		blockBytes, err := dbTx.FetchBlock(&state.Hash)
		if err != nil {
			return err
		}

		block, err := wire.DecodeBlock(bytes.NewReader(blockBytes))
		if err != nil {
			return err
		}

		// As a final consistency check, we'll run through all the
		// nodes which are ancestors of the current chain tip, and mark
		// them as valid if they aren't already marked as such.  This
		// is a safe assumption as all the block before the current tip
		// are valid by definition.
		for iterNode := tip; iterNode != nil; iterNode = iterNode.Parent() {
			// If this isn't already marked as valid in the index, then
			// we'll mark it as valid now to ensure consistency once
			// we're up and running.
			if !iterNode.Status().KnownValid() {
				log.Info().Str("chain", db.Chain().Name()).Msgf("Block %v (height=%v) ancestor of chain tip not marked as valid,"+
					" upgrading to valid for consistency",
					iterNode.GetHash(), iterNode.Height())

				blocksDB.index.SetStatusFlags(iterNode, blocknodes.StatusValid)
			}
		}

		// Initialize the state related to the best block.
		blockSize := uint64(len(blockBytes))
		blockWeight := uint64(chaindata.GetBlockWeight(jaxutil.NewBlock(block)))
		numTxns := uint64(len(block.Transactions))
		stateSnapshot = chaindata.NewBestState(tip,
			blocksDB.bestChain.mmrTree.CurrentRoot(),
			blockSize,
			blockWeight,
			blocksDB.bestChain.mmrTree.CurrenWeight(),
			numTxns,
			state.TotalTxns,
			tip.CalcPastMedianTime(),
			state.LastSerialID,
		)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// As we might have updated the index after it was loaded, we'll
	// attempt to flush the index to the DB. This will only result in a
	// write if the elements are dirty, so it'll usually be a noop.
	return stateSnapshot, blocksDB.index.flushToDB()
}

// createChainState initializes both the database and the chain state to the
// genesis block.  This includes creating the necessary buckets and inserting
// the genesis block, so it must only be called on an uninitialized database.
func createChainState(db database.DB, blocksDB *rBlockStorage) (*chaindata.BestState, error) {
	// Create a new node from the genesis block and set it as the best node.
	genesisBlock := jaxutil.NewBlock(db.Chain().Params().GenesisBlock())
	header := genesisBlock.MsgBlock().Header
	genesisNode := db.Chain().NewNode(header, nil, 0)
	genesisNode.SetStatus(blocknodes.StatusDataStored | blocknodes.StatusValid)
	blocksDB.bestChain.SetTip(genesisNode)

	// Add the new node to the index which is used for faster lookups.
	blocksDB.index.addNode(genesisNode, false)

	// Initialize the state related to the best block.  Since it is the
	// genesis block, use its timestamp for the median time.
	numTxns := uint64(len(genesisBlock.MsgBlock().Transactions))
	blockSize := uint64(genesisBlock.MsgBlock().SerializeSize())
	blockWeight := uint64(chaindata.GetBlockWeight(genesisBlock))

	stateSnapshot := chaindata.NewBestState(genesisNode,
		blocksDB.index.mmrTree.CurrentRoot(),
		blockSize,
		blockWeight,
		blocksDB.index.mmrTree.CurrenWeight(),
		numTxns,
		numTxns,
		time.Unix(genesisNode.Timestamp(), 0),
		0,
	)

	// Create the initial the database chain state including creating the
	// necessary index buckets and inserting the genesis block.
	err := db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		buckets := [][]byte{
			// Create the bucket that houses the block index data.
			chaindata.BlockIndexBucketName,
			// Create the bucket that houses the chain block hash to height index.
			chaindata.HashIndexBucketName,
			// Create the bucket that houses the chain block height to hash index.
			chaindata.HeightIndexBucketName,
			// Create the bucket that houses the spend journal data and store its version.
			chaindata.SpendJournalBucketName,
			// Create the bucket that houses the utxo set and store its
			// version.  Note that the genesis block coinbase transaction is
			// intentionally not inserted here since it is not spendable by
			// consensus rules.
			chaindata.UtxoSetBucketName,
			// Create the bucket that houses the mapping of hash to block serial id and serial id
			chaindata.BlockHashToSerialID,
			// Create the bucket that houses the mapping of block serial id to hash and previouse serial id
			chaindata.SerialIDToPrevBlock,
			chaindata.MMRRootsToHashBucketName,
			chaindata.HashToMMRRootBucketName,
			chaindata.BestChainSerialIDsBucketName,
		}
		if db.Chain().IsBeacon() {
			buckets = append(buckets,
				// Create the bucket that houses the EAD addresses index.
				chaindata.EADAddressesBucketNameV2, // EAD registration is possible only at beacon
				chaindata.ShardCreationsBucketName,
			)
		}

		for _, bucket := range buckets {
			_, err := meta.CreateBucket(bucket)
			if err != nil {
				return err
			}
		}

		err := chaindata.DBPutVersion(dbTx, chaindata.UtxoSetVersionKeyName,
			chaindata.LatestUtxoSetBucketVersion)
		if err != nil {
			return err
		}

		err = chaindata.DBPutHashToSerialIDWithPrev(dbTx, *genesisBlock.Hash(), 0, 0)
		if err != nil {
			return err
		}

		err = chaindata.DBPutVersion(dbTx, chaindata.SpendJournalVersionKeyName,
			chaindata.LatestSpendJournalBucketVersion)
		if err != nil {
			return err
		}

		// Save the genesis block to the block index database.
		err = chaindata.DBStoreBlockNode(dbTx, genesisNode)
		if err != nil {
			return err
		}

		h := genesisNode.GetHash()
		// Add the genesis block hash to height and height to hash
		// mappings to the index.
		err = chaindata.DBPutBlockIndex(dbTx, &h, genesisNode.Height())
		if err != nil {
			return err
		}

		// Store the current best chain state into the database.
		err = chaindata.DBPutBestState(dbTx, stateSnapshot, genesisNode.WorkSum())
		if err != nil {
			return err
		}
		log.Info().Str("chain", db.Chain().Name()).Msgf("Store new genesis: Chain %s Hash %s",
			db.Chain().Name(), genesisBlock.Hash())

		// Store the genesis block into the database.
		err = chaindata.DBStoreBlock(dbTx, genesisBlock)
		if err != nil {
			return err
		}

		if db.Chain().IsBeacon() {
			view := chaindata.NewUtxoViewpoint(db.Chain().IsBeacon())
			_ = view.ConnectTransactions(genesisBlock, nil)

			// Update the utxo set using the state of the utxo view.  This
			// entails removing all of the utxos spent and adding the new
			// ones created by the block.
			err = chaindata.DBPutUtxoView(dbTx, view)
			if err != nil {
				return err
			}

		}
		return nil
	})
	return stateSnapshot, err
}

func fastInitChainState(db database.DB, blocksDB *rBlockStorage) (bool, *chaindata.BestState, error) {
	var lastNode blocknodes.IBlockNode
	var fullScanRequired bool
	var stateSnapshot *chaindata.BestState

	err := db.View(func(dbTx database.Tx) error {
		bestStateData := dbTx.Metadata().Get(chaindata.ChainStateKeyName)
		state, err := chaindata.DeserializeBestChainState(bestStateData)
		if err != nil {
			return err
		}

		bestChain, err := chaindata.DBGetBestChainSerialIDs(dbTx)
		if err != nil {
			return err
		}
		if len(bestChain) == 0 {
			fullScanRequired = true
			return nil
		}

		mmrRoots, err := chaindata.DBGetBlocksMMRRoots(dbTx)
		if err != nil {
			return errors.Wrap(err, "can't get blocks mmr roots")
		}

		log.Info().Str("chain", db.Chain().Name()).Msgf("Loading best chain blocks...")

		blocksDB.index.mmrTree.AllocForFastAdd(uint64(len(bestChain)))
		blocksDB.bestChain.mmrTree.AllocForFastAdd(uint64(len(bestChain)))

		for _, record := range bestChain {
			parent := lastNode

			rawBlock, err := dbTx.FetchBlock(record.Hash)
			if err != nil {
				return errors.Wrap(err, "can't fetch block")
			}

			lastBlock, err := wire.DecodeBlock(bytes.NewBuffer(rawBlock))
			if err != nil {
				return errors.Wrap(err, "can't decode block")
			}

			header := lastBlock.Header

			if parent != nil {
				prevHash := parent.GetHash()
				bph := header.PrevBlockHash()
				if !prevHash.IsEqual(&bph) {
					str := fmt.Sprintf("hash(%s) of parent resolved by mmr(%s) not match with hash(%s) from header",
						prevHash, header.PrevBlocksMMRRoot(), bph)
					return chaindata.AssertError(str)
				}
			}

			// Initialize the block node for the block, connect it,
			// and add it to the block index.
			node := db.Chain().NewNode(header, parent, record.SerialID)
			node.SetStatus(blocknodes.StatusValid)

			root, ok := mmrRoots[node.GetHash()]
			if !ok && node.Height() == 0 {
				root = node.GetHash()
			}

			if !ok && node.Height() > 0 {
				log.Error().Stringer("block_hash", node.GetHash()).
					Int32("height", node.Height()).Msg("mmr root for block not found")
				fullScanRequired = true
				return nil
			}

			node.SetActualMMRRoot(root)

			blocksDB.index.addNode(node, true)
			blocksDB.bestChain.setTip(node, true)
			lastNode = node
		}

		lastHash := lastNode.GetHash()
		if !lastHash.IsEqual(&state.Hash) {
			return chaindata.AssertError(fmt.Sprintf(
				"initChainState: last node hash(%s) height(%d) does not match with chain tip(%s) tip_height(%d)",
				lastHash, lastNode.Height(), state.Hash, state.Height))
		}

		blocksDB.lastSerialID = state.LastSerialID

		// Set the best chain view to the stored best state.
		tip := blocksDB.index.LookupNode(&state.Hash)
		if tip == nil {
			return chaindata.AssertError(fmt.Sprintf(
				"initChainState: cannot find chain tip %s in block index", state.Hash))
		}

		// Load the raw block bytes for the best block.
		blockBytes, err := dbTx.FetchBlock(&state.Hash)
		if err != nil {
			return err
		}

		block, err := wire.DecodeBlock(bytes.NewReader(blockBytes))
		if err != nil {
			return err
		}

		// Initialize the state related to the best block.
		blockSize := uint64(len(blockBytes))
		blockWeight := uint64(chaindata.GetBlockWeight(jaxutil.NewBlock(block)))
		numTxns := uint64(len(block.Transactions))
		stateSnapshot = chaindata.NewBestState(lastNode,
			blocksDB.bestChain.mmrTree.CurrentRoot(),
			blockSize,
			blockWeight,
			blocksDB.bestChain.mmrTree.CurrenWeight(),
			numTxns,
			state.TotalTxns,
			tip.CalcPastMedianTime(),
			state.LastSerialID,
		)
		return nil
	})

	if err != nil || fullScanRequired {
		return fullScanRequired, nil, err
	}

	err = blocksDB.index.mmrTree.RebuildTreeAndAssert()
	if err != nil {
		return true, nil, errors.Wrap(err, "index.mmrTree is inconsistent")
	}
	err = blocksDB.bestChain.mmrTree.RebuildTreeAndAssert()
	if err != nil {
		return true, nil, errors.Wrap(err, "bestChain.mmrTree is inconsistent")
	}

	return false, stateSnapshot, blocksDB.index.flushToDB()
}
