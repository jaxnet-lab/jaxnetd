// Copyright (c) 2015-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"sync"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/mmr"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// blockIndex provides facilities for keeping track of an in-memory index of the
// block chain.  Although the name block chain suggests a single chain of
// blocks, it is actually a tree-shaped structure where any node can have
// multiple children.  However, there can only be one active branch which does
// indeed form a chain from the tip all the way back to the genesis block.
type blockIndex struct {
	// The following fields are set when the instance is created and can't
	// be changed afterwards, so there is no need to protect them with a
	// separate mutex.
	db          database.DB
	chainParams *chaincfg.Params

	sync.RWMutex
	index map[chainhash.Hash]blocknodes.IBlockNode
	dirty map[blocknodes.IBlockNode]struct{}

	// mmrTree an instance of the MMR tree needed to work
	// with the block index and MMR root recalculation.
	// This tree cannot be used as the provider of the main chain root.
	mmrTree mmr.TreeContainer
}

// newBlockIndex returns a new empty instance of a block index.  The index will
// be dynamically populated as block nodes are loaded from the database and
// manually added.
func newBlockIndex(db database.DB, chainParams *chaincfg.Params) *blockIndex {
	return &blockIndex{
		db:          db,
		chainParams: chainParams,
		index:       make(map[chainhash.Hash]blocknodes.IBlockNode),
		dirty:       make(map[blocknodes.IBlockNode]struct{}),
		mmrTree: mmr.TreeContainer{
			BlocksMMRTree: mmr.NewTree(),
			RootToBlock:   map[chainhash.Hash]chainhash.Hash{},
		},
	}
}

// HaveBlock returns whether or not the block index contains the provided hash.
//
// This function is safe for concurrent access.
func (bi *blockIndex) HaveBlock(hash *chainhash.Hash) bool {
	bi.RLock()
	_, hasBlock := bi.index[*hash]
	bi.RUnlock()
	return hasBlock
}

// HashByMMR returns hash of block, that has provided MMR root.
//
// This function is safe for concurrent access.
func (bi *blockIndex) HashByMMR(root chainhash.Hash) (chainhash.Hash, bool) {
	bi.RLock()
	hash, hasBlock := bi.mmrTree.RootToBlock[root]
	bi.RUnlock()
	return hash, hasBlock
}

// LookupNodeByMMRRoot returns the block node identified by the provided MMR Root hash. It will
// return nil if there is no entry for the hash.
//
// This function is safe for concurrent access.
func (bi *blockIndex) LookupNodeByMMRRoot(root chainhash.Hash) blocknodes.IBlockNode {
	bi.RLock()
	hash := bi.mmrTree.RootToBlock[root]
	node := bi.index[hash]
	bi.RUnlock()
	return node
}

// LookupNode returns the block node identified by the provided hash.  It will
// return nil if there is no entry for the hash.
//
// This function is safe for concurrent access.
func (bi *blockIndex) LookupNode(hash *chainhash.Hash) blocknodes.IBlockNode {
	bi.RLock()
	node := bi.index[*hash]
	bi.RUnlock()
	return node
}

// AddNode adds the provided node to the block index and marks it as dirty.
// Duplicate entries are not checked so it is up to caller to avoid adding them.
//
// This function is safe for concurrent access.
func (bi *blockIndex) AddNode(node blocknodes.IBlockNode) {
	bi.Lock()
	bi.addNode(node, false)
	bi.dirty[node] = struct{}{}
	bi.Unlock()
}

// addNode adds the provided node to the block index, but does not mark it as
// dirty. This can be used while initializing the block index.
//
// This function is NOT safe for concurrent access.
func (bi *blockIndex) addNode(node blocknodes.IBlockNode, quickAdd bool) {
	bi.index[node.GetHash()] = node
	if quickAdd {
		bi.mmrTree.SetNodeQuick(node)
		return
	}

	bi.mmrTree.SetNodeToMmrWithReorganization(node)
}

// NodeStatus provides concurrent-safe access to the status field of a node.
//
// This function is safe for concurrent access.
func (bi *blockIndex) NodeStatus(node blocknodes.IBlockNode) blocknodes.BlockStatus {
	bi.RLock()
	status := node.Status()
	bi.RUnlock()
	return status
}

// SetStatusFlags flips the provided status flags on the block node to on,
// regardless of whether they were on or off previously. This does not unset any
// flags currently on.
//
// This function is safe for concurrent access.
func (bi *blockIndex) SetStatusFlags(node blocknodes.IBlockNode, flags blocknodes.BlockStatus) {
	bi.Lock()
	status := node.Status()
	status |= flags
	node.SetStatus(status)
	bi.dirty[node] = struct{}{}
	bi.Unlock()
}

// UnsetStatusFlags flips the provided status flags on the block node to off,
// regardless of whether they were on or off previously.
//
// This function is safe for concurrent access.
func (bi *blockIndex) UnsetStatusFlags(node blocknodes.IBlockNode, flags blocknodes.BlockStatus) {
	bi.Lock()
	status := node.Status()
	status &^= flags
	node.SetStatus(status)

	bi.dirty[node] = struct{}{}
	bi.Unlock()
}

func (bi *blockIndex) MMRTreeRoot() chainhash.Hash {
	bi.RLock()
	root := bi.mmrTree.CurrentRoot()
	bi.RUnlock()
	return root
}

// flushToDB writes all dirty block nodes to the database. If all writes
// succeed, this clears the dirty set.
func (bi *blockIndex) flushToDB() error {
	bi.Lock()
	if len(bi.dirty) == 0 {
		bi.Unlock()
		return nil
	}

	err := bi.db.Update(func(dbTx database.Tx) error {
		for node := range bi.dirty {
			err := chaindata.RepoTx(dbTx).StoreBlockNode(node)
			if err != nil {
				return err
			}
		}
		return nil
	})

	// If write was successful, clear the dirty set.
	if err == nil {
		bi.dirty = make(map[blocknodes.IBlockNode]struct{})
	}

	bi.Unlock()
	return err
}

func (bi *blockIndex) setFromCache(cache *blockIndexCache, tip blocknodes.IBlockNode) error {
	bi.Lock()
	defer bi.Unlock()
	for hash := range cache.index {
		bi.index[hash] = cache.index[hash]
	}
	for node := range cache.dirty {
		bi.dirty[node] = struct{}{}
	}

	bi.mmrTree.SetNodeToMmrWithReorganization(tip)
	return bi.mmrTree.RebuildTreeAndAssert()
}

type blockIndexCache struct {
	sync.RWMutex
	index map[chainhash.Hash]blocknodes.IBlockNode
	dirty map[blocknodes.IBlockNode]struct{}
}

func newBlockIndexCache() *blockIndexCache {
	return &blockIndexCache{
		index: make(map[chainhash.Hash]blocknodes.IBlockNode),
		dirty: make(map[blocknodes.IBlockNode]struct{}),
	}
}

// LookupNode returns the block node identified by the provided hash.  It will
// return nil if there is no entry for the hash.
//
// This function is safe for concurrent access.
func (bi *blockIndexCache) LookupNode(hash *chainhash.Hash) blocknodes.IBlockNode {
	bi.RLock()
	node := bi.index[*hash]
	bi.RUnlock()
	return node
}

// AddNode adds the provided node to the block index and marks it as dirty.
// Duplicate entries are not checked so it is up to caller to avoid adding them.
//
// This function is safe for concurrent access.
func (bi *blockIndexCache) AddNode(node blocknodes.IBlockNode) {
	bi.Lock()
	bi.index[node.GetHash()] = node
	bi.dirty[node] = struct{}{}
	bi.Unlock()
}
