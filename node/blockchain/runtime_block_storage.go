/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package blockchain

import (
	"container/list"
	"fmt"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// rBlockStorage is main housekeeper for block & block headers storage.
type rBlockStorage struct {
	// These fields are related to the memory block index.  They both have
	// their own locks, however they are often also protected by the chain
	// lock to help prevent logic races when blocks are being processed.
	//
	// index houses the entire block index in memory.  The block index is
	// a tree-shaped structure.
	//
	// bestChain tracks the current active chain by making use of an
	// efficient chain view into the block index.
	index     *blockIndex
	bestChain *chainView

	// These fields are related to handling of orphan blocks.  They are
	// protected by a combination of the chain lock and the orphan lock.
	orphanIndex  orphanIndex
	lastSerialID int64
}

func newBlocksStorage(db database.DB, params *chaincfg.Params) rBlockStorage {
	return rBlockStorage{
		index:     newBlockIndex(db, params),
		bestChain: newChainView(nil),
		orphanIndex: orphanIndex{
			orphans:          make(map[chainhash.Hash]*orphanBlock),
			actualMMRToBlock: make(map[chainhash.Hash]*orphanBlock),
			mmrRootsOrphans:  make(map[chainhash.Hash][]*orphanBlock),
		},
	}
}

// blockExists determines whether a block with the given hash exists either in
// the main chain or any side chains.
//
// This function is safe for concurrent access.
// Returns: blockExist, blockOrphan, error
func (storage *rBlockStorage) blockExists(db database.DB, hash *chainhash.Hash) (bool, error) {
	// Check block index first (could be main chain or side chain blocks).
	if storage.index.HaveBlock(hash) {
		return true, nil
	}

	// Check in the database.
	var exists bool
	err := db.View(func(dbTx database.Tx) error {
		var err error
		exists, err = dbTx.HasBlock(hash)
		if err != nil || !exists {
			return err
		}

		// Ignore side chain blocks in the database.  This is necessary
		// because there is not currently any record of the associated
		// block index data such as its block height, so it's not yet
		// possible to efficiently load the block and do anything useful
		// with it.
		//
		// Ultimately the entire block index should be serialized
		// instead of only the current main chain so it can be consulted
		// directly.
		_, err = chaindata.DBFetchHeightByHash(dbTx, hash)
		if chaindata.IsNotInMainChainErr(err) {
			exists = false
			return nil
		}
		return err
	})
	return exists, err
}

func (storage *rBlockStorage) getBlockParent(prevMMRRoot chainhash.Hash) (blocknodes.IBlockNode, bool, error) {
	prevNode := storage.index.LookupNodeByMMRRoot(prevMMRRoot)
	if prevNode == nil {
		str := fmt.Sprintf("previous block %s is unknown", prevMMRRoot)
		return nil, false, chaindata.NewRuleError(chaindata.ErrPreviousBlockUnknown, str)
	}

	if storage.index.NodeStatus(prevNode).KnownInvalid() {
		str := fmt.Sprintf("previous block %s is known to be invalid", prevMMRRoot)
		return nil, true, chaindata.NewRuleError(chaindata.ErrInvalidAncestorBlock, str)
	}

	return prevNode, true, nil
}

// getBlockParentHash returns: correspondingBlockHash, found, inMainChain
func (storage *rBlockStorage) getBlockParentHash(prevMMRRoot chainhash.Hash) (chainhash.Hash, bool, bool) {
	inMainChain := true
	prevHash, found := storage.bestChain.HashByMMR(prevMMRRoot)
	if !found {
		prevHash, found = storage.index.HashByMMR(prevMMRRoot)
		inMainChain = false
	}

	return prevHash, found, inMainChain
}

func (storage *rBlockStorage) getMMRRootForHash(blockHash chainhash.Hash) (chainhash.Hash, bool) {
	leaf, found := storage.bestChain.mmrTree.LeafByHash(blockHash)
	if !found {
		leaf, found = storage.index.mmrTree.LeafByHash(blockHash)
	}

	return leaf.ActualRoot, found
}

// getReorganizeNodes finds the fork point between the main chain and the passed
// node and returns a list of block nodes that would need to be detached from
// the main chain and a list of block nodes that would need to be attached to
// the fork point (which will be the end of the main chain after detaching the
// returned list of block nodes) in order to reorganize the chain such that the
// passed node is the new end of the main chain.  The lists will be empty if the
// passed node is not on a side chain.
//
// This function may modify node statuses in the block index without flushing.
//
// This function MUST be called with the chain state lock held (for reads).
// nolint: forcetypeassert
func (storage *rBlockStorage) getReorganizeNodes(node blocknodes.IBlockNode) (*list.List, *list.List) {
	attachNodes := list.New()
	detachNodes := list.New()

	// Do not reorganize to a known invalid chain. Ancestors deeper than the
	// direct parent are checked below but this is a quick check before doing
	// more unnecessary work.
	if storage.index.NodeStatus(node.Parent()).KnownInvalid() {
		storage.index.SetStatusFlags(node, blocknodes.StatusInvalidAncestor)
		return detachNodes, attachNodes
	}

	// Find the fork point (if any) adding each block to the list of nodes
	// to attach to the main tree.  Push them onto the list in reverse order
	// so they are attached in the appropriate order when iterating the list
	// later.
	forkNode := storage.bestChain.FindFork(node)
	invalidChain := false
	for n := node; n != nil && n != forkNode; n = n.Parent() {
		if storage.index.NodeStatus(n).KnownInvalid() {
			invalidChain = true
			break
		}
		attachNodes.PushFront(n)
	}

	// If any of the node's ancestors are invalid, unwind attachNodes, marking
	// each one as invalid for future reference.
	if invalidChain {
		var next *list.Element
		for e := attachNodes.Front(); e != nil; e = next {
			next = e.Next()
			n := attachNodes.Remove(e).(blocknodes.IBlockNode)
			storage.index.SetStatusFlags(n, blocknodes.StatusInvalidAncestor)
		}
		return detachNodes, attachNodes
	}

	// Start from the end of the main chain and work backwards until the
	// common ancestor adding each block to the list of nodes to detach from
	// the main chain.
	for n := storage.bestChain.Tip(); n != nil && n != forkNode; n = n.Parent() {
		detachNodes.PushBack(n)
	}

	return detachNodes, attachNodes
}
