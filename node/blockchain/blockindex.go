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
	mmrTree mmrContainer
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
		mmrTree: mmrContainer{
			BlocksMMRTree:  mmr.NewTree(),
			mmrRootToBlock: map[chainhash.Hash]chainhash.Hash{},
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
	hash, hasBlock := bi.mmrTree.mmrRootToBlock[root]
	bi.RUnlock()
	return hash, hasBlock
}

// LookupNodeByMMRRoot returns the block node identified by the provided MMR Root hash. It will
// return nil if there is no entry for the hash.
//
// This function is safe for concurrent access.
func (bi *blockIndex) LookupNodeByMMRRoot(root chainhash.Hash) blocknodes.IBlockNode {
	bi.RLock()
	hash := bi.mmrTree.mmrRootToBlock[root]
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
	bi.addNode(node)
	bi.dirty[node] = struct{}{}
	bi.Unlock()
}

// addNode adds the provided node to the block index, but does not mark it as
// dirty. This can be used while initializing the block index.
//
// This function is NOT safe for concurrent access.
func (bi *blockIndex) addNode(node blocknodes.IBlockNode) {
	bi.index[node.GetHash()] = node

	bi.mmrTree.setNodeToMmrWithReorganization(node)
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
			err := chaindata.DBStoreBlockNode(dbTx, node)
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

type mmrContainer struct {
	*mmr.BlocksMMRTree
	// mmrRootToBlock stores all known pairs of the mmr_root and corresponding block,
	// which was the last leaf in the tree for this root.
	// Here is stored all roots for the main chain and orphans.
	mmrRootToBlock map[chainhash.Hash]chainhash.Hash
}

func (mmrTree *mmrContainer) setNodeToMmrWithReorganization(node blocknodes.IBlockNode) {
	prevNodesMMRRoot := node.Header().BlocksMerkleMountainRoot()
	currentMMRRoot := mmrTree.CurrentRoot()

	// 1) Good Case: if a new node is next in the current chain,
	// then just push it to the MMR tree as the last leaf.
	if prevNodesMMRRoot.IsEqual(&currentMMRRoot) {
		mmrTree.AddBlock(node.GetHash(), node.Difficulty())
		mmrTree.mmrRootToBlock[mmrTree.CurrentRoot()] = node.GetHash()
		node.SetActualMMRRoot(mmrTree.CurrentRoot())
		return
	}

	lifoToAdd := []mmr.Leaf{
		{Hash: node.GetHash(), Weight: node.Difficulty()},
	}

	// 2) OrphanAdd Case: if a node is not next in the current chain,
	// then looking for the first ancestor (<fork root>) that is present in current chain,
	// resetting MMR tree state to this <fork root> as the last leaf
	// and adding all blocks between <fork root> and a new node.
	iterNode := node.Parent()
	iterMMRRoot := node.Header().BlocksMerkleMountainRoot()
	for iterNode != nil {
		prevHash := iterNode.GetHash()
		bNode, topPresent := mmrTree.LookupNodeByRoot(iterMMRRoot)
		if topPresent {
			if !bNode.Hash.IsEqual(&prevHash) || iterNode.Height() != int32(bNode.ID) {
				// todo: impossible in normal world situation
				return
			}

			mmrTree.ResetRootTo(bNode.Hash, int32(bNode.ID))
			break
		}

		lifoToAdd = append(lifoToAdd, mmr.Leaf{Hash: iterNode.GetHash(), Weight: iterNode.Difficulty()})

		iterMMRRoot = iterNode.Header().BlocksMerkleMountainRoot()
		iterNode = iterNode.Parent()
	}

	for i := len(lifoToAdd) - 1; i >= 0; i-- {
		bNode := lifoToAdd[i]
		mmrTree.AddBlock(bNode.Hash, bNode.Weight)
		mmrTree.mmrRootToBlock[mmrTree.CurrentRoot()] = node.GetHash()
		node.SetActualMMRRoot(mmrTree.CurrentRoot())
	}
}
