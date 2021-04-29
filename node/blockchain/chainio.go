// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package blockchain

import (
	"bytes"
	"fmt"
	"time"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/node/chaindata"
	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
)

// FetchSpendJournal attempts to retrieve the spend journal, or the set of
// outputs spent for the target block. This provides a view of all the outputs
// that will be consumed once the target block is connected to the end of the
// main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) FetchSpendJournal(targetBlock *btcutil.Block) ([]chaindata.SpentTxOut, error) {
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
	genesisBlock := btcutil.NewBlock(b.chain.GenesisBlock())
	genesisBlock.SetHeight(0)
	header := genesisBlock.MsgBlock().Header
	node := b.chain.NewNode(header, nil)
	node.SetStatus(blocknode.StatusDataStored | blocknode.StatusValid)
	b.bestChain.SetTip(node)

	// Add the new node to the index which is used for faster lookups.
	b.index.addNode(node)

	// Initialize the state related to the best block.  Since it is the
	// genesis block, use its timestamp for the median time.
	numTxns := uint64(len(genesisBlock.MsgBlock().Transactions))
	blockSize := uint64(genesisBlock.MsgBlock().SerializeSize())
	blockWeight := uint64(chaindata.GetBlockWeight(genesisBlock))
	b.stateSnapshot = chaindata.NewBestState(node, blockSize, blockWeight, numTxns,
		numTxns, time.Unix(node.Timestamp(), 0))

	// Create the initial the database chain state including creating the
	// necessary index buckets and inserting the genesis block.
	err := b.db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()

		// Create the bucket that houses the block index data.
		_, err := meta.CreateBucket(chaindata.BlockIndexBucketName)
		if err != nil {
			return err
		}

		// Create the bucket that houses the chain block hash to height
		// index.
		_, err = meta.CreateBucket(chaindata.HashIndexBucketName)
		if err != nil {
			return err
		}

		// Create the bucket that houses the chain block height to hash
		// index.
		_, err = meta.CreateBucket(chaindata.HeightIndexBucketName)
		if err != nil {
			return err
		}

		// Create the bucket that houses the MerkleMountainRange for Merged Mining Tree..
		_, err = meta.CreateBucket(chaindata.ShardsMMRBucketName)
		if err != nil {
			return err
		}

		// Create the bucket that houses the EAD addresses index.
		_, err = meta.CreateBucket(chaindata.EADAddressesBucketName)
		if err != nil {
			return err
		}

		// Create the bucket that houses the spend journal data and
		// store its version.
		_, err = meta.CreateBucket(chaindata.SpendJournalBucketName)
		if err != nil {
			return err
		}
		err = chaindata.DBPutVersion(dbTx, chaindata.UtxoSetVersionKeyName,
			chaindata.LatestUtxoSetBucketVersion)
		if err != nil {
			return err
		}

		// Create the bucket that houses the utxo set and store its
		// version.  Note that the genesis block coinbase transaction is
		// intentionally not inserted here since it is not spendable by
		// consensus rules.
		_, err = meta.CreateBucket(chaindata.UtxoSetBucketName)
		if err != nil {
			return err
		}

		// Create the bucket that houses block last_serial_id
		_, err = meta.CreateBucket(chaindata.BlockLastSerialID)
		if err != nil {
			return err
		}

		// Create the bucket that houses the mapping of hash to block serial id and serial id
		_, err = meta.CreateBucket(chaindata.BlockHashSerialID)
		if err != nil {
			return err
		}

		// // Create the bucket that houses the mapping of block serial id to hash and serial id
		// _, err = meta.CreateBucket(chaindata.BlockSerialIDHash)
		// if err != nil {
		// 	return err
		// }

		// Create the bucket that houses the mapping of block serial id to hash and previouse serial id
		_, err = meta.CreateBucket(chaindata.BlockSerialIDHashPrevSerialID)
		if err != nil {
			return err
		}

		err = chaindata.DBPutLastSerialID(dbTx, 0)
		if err != nil {
			return err
		}

		err = chaindata.DBPutBlockHashSerialID(dbTx, genesisBlock.Hash(), 0)
		if err != nil {
			return err
		}

		err = chaindata.DBPutBlockSerialIDHash(dbTx, genesisBlock.Hash(), 0)
		if err != nil {
			return err
		}

		err = chaindata.DBPutBlockSerialIDHashPrevSerialID(dbTx, genesisBlock.Hash(), 0, -1)
		if err != nil {
			return err
		}

		err = chaindata.DBPutVersion(dbTx, chaindata.SpendJournalVersionKeyName,
			chaindata.LatestSpendJournalBucketVersion)
		if err != nil {
			return err
		}

		// Save the genesis block to the block index database.
		err = chaindata.DBStoreBlockNode(b.chain, dbTx, node)
		if err != nil {
			return err
		}

		h := node.GetHash()
		// Add the genesis block hash to height and height to hash
		// mappings to the index.
		err = chaindata.DBPutBlockIndex(dbTx, &h, node.Height())
		if err != nil {
			return err
		}

		// Store the current best chain state into the database.
		err = chaindata.DBPutBestState(dbTx, b.stateSnapshot, node.WorkSum())
		if err != nil {
			return err
		}
		log.Info().Msgf("Store new genesis: Chain %s Hash %s", b.chain.Name(), genesisBlock.Hash())
		// Store the genesis block into the database.
		return chaindata.DBStoreBlock(dbTx, genesisBlock)
	})
	return err
}

// initChainState attempts to load and initialize the chain state from the
// database.  When the db does not yet contain any chain state, both it and the
// chain state are initialized to the genesis block.
func (b *BlockChain) initChainState() error {
	// Determine the state of the chain database. We may need to initialize
	// everything from scratch or upgrade certain buckets.
	var initialized, hasBlockIndex bool
	err := b.db.View(func(dbTx database.Tx) error {
		initialized = dbTx.Metadata().Get(chaindata.ChainStateKeyName) != nil
		hasBlockIndex = dbTx.Metadata().Bucket(chaindata.BlockIndexBucketName) != nil
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

	if !hasBlockIndex {
		err := chaindata.MigrateBlockIndex(b.db)
		if err != nil {
			return nil
		}
	}

	// Attempt to load the chain state from the database.
	err = b.db.View(func(dbTx database.Tx) error {
		// Fetch the stored chain state from the database metadata.
		// When it doesn't exist, it means the database hasn't been
		// initialized for use with chain yet, so break out now to allow
		// that to happen under a writable database transaction.
		serializedData := dbTx.Metadata().Get(chaindata.ChainStateKeyName)
		log.Trace().Msgf("Serialized chain state: %x", serializedData)
		state, err := chaindata.DeserializeBestChainState(serializedData)
		if err != nil {
			return err
		}

		// Load all of the headers from the data for the known best
		// chain and construct the block index accordingly.  Since the
		// number of nodes are already known, perform a single alloc
		// for them versus a whole bunch of little ones to reduce
		// pressure on the GC.
		log.Info().Msgf("Loading block index...")

		blockIndexBucket := dbTx.Metadata().Bucket(chaindata.BlockIndexBucketName)

		var i int32
		var lastNode blocknode.IBlockNode
		cursor := blockIndexBucket.Cursor()
		for ok := cursor.First(); ok; ok = cursor.Next() {
			header, status, err := chaindata.DeserializeBlockRow(b.chain, cursor.Value())
			if err != nil {
				return err
			}

			// Determine the parent block node. Since we iterate block headers
			// in order of height, if the blocks are mostly linear there is a
			// very good chance the previous header processed is the parent.
			var parent blocknode.IBlockNode
			if lastNode == nil {
				blockHash := header.BlockHash()
				if !blockHash.IsEqual(b.chain.Params().GenesisHash) {
					return chaindata.AssertError(fmt.Sprintf(
						"initChainState: Expected first entry in block index to be genesis block: expected %s, found %s",
						b.chainParams.GenesisHash, blockHash))
				}
			} else if header.PrevBlock() == lastNode.GetHash() {
				// Since we iterate block headers in order of height, if the
				// blocks are mostly linear there is a very good chance the
				// previous header processed is the parent.
				parent = lastNode
			} else {
				prev := header.PrevBlock()
				parent = b.index.LookupNode(&prev)
				if parent == nil {
					return chaindata.AssertError(fmt.Sprintf(
						"initChainState: Could not find parent for block %s", header.BlockHash()))
				}
			}

			// Initialize the block node for the block, connect it,
			// and add it to the block index.
			node := b.chain.NewNode(header, parent)
			node.SetStatus(status)

			b.index.addNode(node)

			lastNode = node
			i++
		}

		// Set the best chain view to the stored best state.
		tip := b.index.LookupNode(&state.Hash)
		if tip == nil {
			return chaindata.AssertError(fmt.Sprintf(
				"initChainState: cannot find chain tip %s in block index", state.Hash))
		}
		b.bestChain.SetTip(tip)

		// Load the raw block bytes for the best block.
		blockBytes, err := dbTx.FetchBlock(&state.Hash)
		if err != nil {
			return err
		}
		block := b.chain.EmptyBlock()
		err = block.Deserialize(bytes.NewReader(blockBytes))
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
				log.Info().Msgf("Block %v (height=%v) ancestor of chain tip not marked as valid,"+
					" upgrading to valid for consistency",
					iterNode.GetHash(), iterNode.Height())

				b.index.SetStatusFlags(iterNode, blocknode.StatusValid)
			}
		}

		// Initialize the state related to the best block.
		blockSize := uint64(len(blockBytes))
		blockWeight := uint64(chaindata.GetBlockWeight(btcutil.NewBlock(&block)))
		numTxns := uint64(len(block.Transactions))
		b.stateSnapshot = chaindata.NewBestState(tip, blockSize, blockWeight,
			numTxns, state.TotalTxns, tip.CalcPastMedianTime())

		return nil
	})
	if err != nil {
		return err
	}

	// As we might have updated the index after it was loaded, we'll
	// attempt to flush the index to the DB. This will only result in a
	// write if the elements are dirty, so it'll usually be a noop.
	return b.index.flushToDB()
}

// BlockByHeight returns the block at the given height in the main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockByHeight(blockHeight int32) (*btcutil.Block, error) {
	// Lookup the block height in the best chain.
	node := b.bestChain.NodeByHeight(blockHeight)
	if node == nil {
		str := fmt.Sprintf("no block at height %d exists", blockHeight)
		return nil, chaindata.ErrNotInMainChain(str)
	}

	// Load the block from the database and return it.
	var block *btcutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		block, err = chaindata.DBFetchBlockByNode(b.chain, dbTx, node)
		return err
	})
	return block, err
}

// BlockByHash returns the block from the main chain with the given hash with
// the appropriate chain height set.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockByHash(hash *chainhash.Hash) (*btcutil.Block, error) {
	// Lookup the block hash in block index and ensure it is in the best
	// chain.
	node := b.index.LookupNode(hash)
	if node == nil || !b.bestChain.Contains(node) {
		str := fmt.Sprintf("block %s is not in the main chain", hash)
		return nil, chaindata.ErrNotInMainChain(str)
	}

	// Load the block from the database and return it.
	var block *btcutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		block, err = chaindata.DBFetchBlockByNode(b.chain, dbTx, node)
		return err
	})
	return block, err
}
