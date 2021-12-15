// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"fmt"
	"time"

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

// createChainState initializes both the database and the chain state to the
// genesis block.  This includes creating the necessary buckets and inserting
// the genesis block, so it must only be called on an uninitialized database.
func (b *BlockChain) createChainState() error {
	// Create a new node from the genesis block and set it as the best node.
	genesisBlock := jaxutil.NewBlock(b.chain.Params().GenesisBlock())
	header := genesisBlock.MsgBlock().Header
	genesisNode := b.chain.NewNode(header, nil, 0)
	genesisNode.SetStatus(blocknodes.StatusDataStored | blocknodes.StatusValid)
	b.blocksDB.bestChain.SetTip(genesisNode)

	// Add the new node to the index which is used for faster lookups.
	b.blocksDB.index.addNode(genesisNode)

	// Initialize the state related to the best block.  Since it is the
	// genesis block, use its timestamp for the median time.
	numTxns := uint64(len(genesisBlock.MsgBlock().Transactions))
	blockSize := uint64(genesisBlock.MsgBlock().SerializeSize())
	blockWeight := uint64(chaindata.GetBlockWeight(genesisBlock))

	b.stateSnapshot = chaindata.NewBestState(genesisNode,
		b.blocksDB.index.mmrTree.CurrentRoot(),
		blockSize,
		blockWeight,
		b.blocksDB.index.mmrTree.CurrenWeight(),
		numTxns,
		numTxns,
		time.Unix(genesisNode.Timestamp(), 0),
		0,
	)

	// Create the initial the database chain state including creating the
	// necessary index buckets and inserting the genesis block.
	err := b.db.Update(func(dbTx database.Tx) error {
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
		if b.chain.IsBeacon() {
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
		err = chaindata.DBPutBestState(dbTx, b.stateSnapshot, genesisNode.WorkSum())
		if err != nil {
			return err
		}
		log.Info().Str("chain", b.chain.Name()).Msgf("Store new genesis: Chain %s Hash %s", b.chain.Name(), genesisBlock.Hash())

		// Store the genesis block into the database.
		err = chaindata.DBStoreBlock(dbTx, genesisBlock)
		if err != nil {
			return err
		}

		if b.chain.IsBeacon() {
			view := chaindata.NewUtxoViewpoint(b.chain.IsBeacon())
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
	return err
}

// initChainState attempts to load and initialize the chain state from the
// database.  When the db does not yet contain any chain state, both it and the
// chain state are initialized to the genesis block.
// nolint: gocritic
func (b *BlockChain) initChainState() error {
	// Determine the state of the chain database. We may need to initialize
	// everything from scratch or upgrade certain buckets.
	var (
		initialized             bool
		bestChainSnapshotExists bool
	)

	err := b.db.View(func(dbTx database.Tx) error {
		initialized = dbTx.Metadata().Get(chaindata.ChainStateKeyName) != nil
		bestChainSnapshotExists = dbTx.Metadata().Bucket(chaindata.BestChainSerialIDsBucketName) != nil
		return nil
	})
	if err != nil {
		return err
	}

	if !initialized {
		// At this point the database has not already been initialized, so
		// initialize both it and the chain state to the genesis block.
		return b.createChainState()
	}

	if bestChainSnapshotExists && !b.dbFullRescan {
		rescanRequired, err := b.fastInitChainState()
		if !rescanRequired || err != nil {
			return err
		}
	}

	// Attempt to load the chain state from the database.
	err = b.db.View(func(dbTx database.Tx) error {
		// Fetch the stored chain state from the database metadata.
		// When it doesn't exist, it means the database hasn't been
		// initialized for use with chain yet, so break out now to allow
		// that to happen under a writable database transaction.
		serializedData := dbTx.Metadata().Get(chaindata.ChainStateKeyName)
		log.Trace().Str("chain", b.chain.Name()).Msgf("Serialized chain state: %x", serializedData)
		state, err := chaindata.DeserializeBestChainState(serializedData)
		if err != nil {
			return err
		}

		// Load all of the headers from the data for the known best
		// chain and construct the block index accordingly.  Since the
		// number of nodes are already known, perform a single alloc
		// for them versus a whole bunch of little ones to reduce
		// pressure on the GC.
		log.Info().Str("chain", b.chain.Name()).Msgf("Loading block index...")

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

				if !blockHash.IsEqual(b.chain.Params().GenesisHash()) {
					return chaindata.AssertError(fmt.Sprintf(
						"initChainState: Expected first entry in block index to be genesis block: expected %s, found %s",
						b.chain.Params().GenesisHash(), blockHash))
				}
			} else if header.PrevBlocksMMRRoot() == b.blocksDB.index.MMRTreeRoot() {
				// Since we iterate block headers in order of height, if the
				// blocks are mostly linear there is a very good chance the
				// previous header processed is the parent.
				parent = lastNode
			} else {
				prev := header.PrevBlocksMMRRoot()
				parent = b.blocksDB.index.LookupNodeByMMRRoot(prev)
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
			node := b.chain.NewNode(header, parent, blockSerialID)
			node.SetStatus(status)

			b.blocksDB.index.addNode(node)

			lastNode = node
			i++
		}

		b.blocksDB.lastSerialID = state.LastSerialID

		// Set the best chain view to the stored best state.
		tip := b.blocksDB.index.LookupNode(&state.Hash)
		if tip == nil {
			return chaindata.AssertError(fmt.Sprintf(
				"initChainState: cannot find chain tip %s in block index", state.Hash))
		}
		b.blocksDB.bestChain.SetTip(tip)

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
				log.Info().Str("chain", b.chain.Name()).Msgf("Block %v (height=%v) ancestor of chain tip not marked as valid,"+
					" upgrading to valid for consistency",
					iterNode.GetHash(), iterNode.Height())

				b.blocksDB.index.SetStatusFlags(iterNode, blocknodes.StatusValid)
			}
		}

		// Initialize the state related to the best block.
		blockSize := uint64(len(blockBytes))
		blockWeight := uint64(chaindata.GetBlockWeight(jaxutil.NewBlock(block)))
		numTxns := uint64(len(block.Transactions))
		b.stateSnapshot = chaindata.NewBestState(tip,
			b.blocksDB.bestChain.mmrTree.CurrentRoot(),
			blockSize,
			blockWeight,
			b.blocksDB.bestChain.mmrTree.CurrenWeight(),
			numTxns,
			state.TotalTxns,
			tip.CalcPastMedianTime(),
			state.LastSerialID,
		)
		return nil
	})
	if err != nil {
		return err
	}

	// As we might have updated the index after it was loaded, we'll
	// attempt to flush the index to the DB. This will only result in a
	// write if the elements are dirty, so it'll usually be a noop.
	return b.blocksDB.index.flushToDB()
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

func (b *BlockChain) fastInitChainState() (bool, error) {
	var lastNode blocknodes.IBlockNode
	var fullScanRequired bool
	err := b.db.View(func(dbTx database.Tx) error {
		serialIDsData, err := chaindata.DBGetSerialIDsList(dbTx)
		if err != nil {
			return err
		}
		if len(serialIDsData) == 0 {
			fullScanRequired = true
			return nil
		}

		serializedData := dbTx.Metadata().Get(chaindata.ChainStateKeyName)
		log.Trace().Str("chain", b.chain.Name()).Msgf("Serialized chain state: %x", serializedData)
		state, err := chaindata.DeserializeBestChainState(serializedData)
		if err != nil {
			return err
		}

		var hashes = make([]chainhash.Hash, len(serialIDsData))
		for i := range serialIDsData {
			hashes[i] = *serialIDsData[i].Hash
		}

		log.Info().Str("chain", b.chain.Name()).Msgf("Loading best chain blocks...")

		bls, err := dbTx.FetchBlocks(hashes)
		if err != nil {
			return err
		}

		var blocks []wire.BlockHeader
		for i := range bls {
			a, err := wire.DecodeHeader(bytes.NewReader(bls[i]))
			if err != nil {
				return err
			}
			blocks = append(blocks, a)
		}

		for i := range blocks {
			parent := lastNode
			if parent != nil {
				prevHash := parent.GetHash()
				bph := blocks[i].PrevBlockHash()
				if !prevHash.IsEqual(&bph) {
					str := fmt.Sprintf("hash(%s) of parent resolved by mmr(%s) not match with hash(%s) from header",
						prevHash, blocks[i].PrevBlocksMMRRoot(), bph)
					return chaindata.AssertError(str)
				}
			}

			// Initialize the block node for the block, connect it,
			// and add it to the block index.
			node := b.chain.NewNode(blocks[i], parent, serialIDsData[i].SerialID)
			node.SetStatus(blocknodes.StatusValid)

			b.blocksDB.index.addNode(node)

			lastNode = node
		}

		b.blocksDB.lastSerialID = state.LastSerialID

		// Set the best chain view to the stored best state.
		tip := b.blocksDB.index.LookupNode(&state.Hash)
		if tip == nil {
			return chaindata.AssertError(fmt.Sprintf(
				"initChainState: cannot find chain tip %s in block index", state.Hash))
		}
		b.blocksDB.bestChain.SetTip(tip)

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
		b.stateSnapshot = chaindata.NewBestState(tip,
			b.blocksDB.bestChain.mmrTree.CurrentRoot(),
			blockSize,
			blockWeight,
			b.blocksDB.bestChain.mmrTree.CurrenWeight(),
			numTxns,
			state.TotalTxns,
			tip.CalcPastMedianTime(),
			state.LastSerialID,
		)
		return nil
	})

	if err != nil || fullScanRequired {
		return fullScanRequired, err
	}

	return false, b.blocksDB.index.flushToDB()
}
