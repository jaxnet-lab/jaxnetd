/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type BlocksMMRTree struct {
	sync.RWMutex

	// nextHeight stores number of TreeNode.
	// nextHeight - 1 is last height in chain.
	nextHeight  int32
	chainWeight *big.Int
	lastNode    *TreeNode
	rootHash    chainhash.Hash

	//// nodes is a representation of Merkle Mountain Range tree.
	//// ID starts from 0.
	//nodes []*TreeNode
	store *treeStore
	// rootToHeight is an association of mmr_root and corresponding block_node,
	// that was last in the chain for this root
	rootToHeight map[chainhash.Hash]int32
	heightToRoot map[int32]chainhash.Hash
	// hashToHeight is a map of hashes and their IDs, ID eq to height of block in chain.
	hashToHeight map[chainhash.Hash]int32
}

// nolint: gomnd
func NewTree() *BlocksMMRTree {
	return &BlocksMMRTree{
		lastNode:     &TreeNode{},
		store:        newTreeStore(1),
		rootToHeight: make(map[chainhash.Hash]int32, 4096),
		hashToHeight: make(map[chainhash.Hash]int32, 4096),
		heightToRoot: make(map[int32]chainhash.Hash, 4096),
		chainWeight:  big.NewInt(0),
	}
}

func (t *BlocksMMRTree) MarshalJSON() ([]byte, error) {
	type dto struct {
		NodeCount    int32
		ChainWeight  string
		Nodes        [][]*TreeNode
		RootToHeight map[string]int32
		HashToHeight map[string]int32
		HeightToRoot map[int32]string
	}
	d := dto{
		NodeCount:    t.nextHeight,
		ChainWeight:  t.chainWeight.String(),
		Nodes:        t.store.nodes,
		RootToHeight: make(map[string]int32, len(t.rootToHeight)),
		HashToHeight: make(map[string]int32, len(t.hashToHeight)),
		HeightToRoot: make(map[int32]string, len(t.heightToRoot)),
	}

	for top, nodeID := range t.rootToHeight {
		d.RootToHeight[top.String()] = nodeID
	}

	for hash, nodeID := range t.hashToHeight {
		d.HashToHeight[hash.String()] = nodeID
	}

	for height, root := range t.heightToRoot {
		d.HeightToRoot[height] = root.String()
	}

	return json.Marshal(d)
}

// AppendBlock adds block as latest leaf, increases height and rebuild tree.
func (t *BlocksMMRTree) AppendBlock(hash chainhash.Hash, difficulty *big.Int) {
	t.Lock()
	t.appendBlock(hash, difficulty)
	t.Unlock()
}

func (t *BlocksMMRTree) appendBlock(hash chainhash.Hash, difficulty *big.Int) {
	_, ok := t.hashToHeight[hash]
	if ok {
		return
	}

	node := &TreeNode{
		Hash:   hash,
		Weight: difficulty,
		Height: t.nextHeight,
	}

	t.store.appendLeaf(int(node.Height), node)
	t.rootHash = t.store.calcRoot().Hash
	t.heightToRoot[node.Height] = t.rootHash
	t.hashToHeight[hash] = node.Height
	t.rootToHeight[t.rootHash] = node.Height

	t.nextHeight++
	t.lastNode = node
	t.chainWeight = t.chainWeight.Add(t.chainWeight, difficulty)
}

// PreAllocateTree allocates tree containers to hold expected number of blocks.
//
// IMPORTANT! this function is not safe!
//
// PreAllocateTree should be used only on empty tree instances and only for quick block adding.
// Quick Block Adding must be done like this:
//
//  tree := NewTree()
//  tree.PreAllocateTree(n)
//  for _, block := range blocks {
//     tree.AddBlockWithoutRebuild(....)
//  }
//  err := tree.RebuildTreeAndAssert()
func (t *BlocksMMRTree) PreAllocateTree(blockCount int) {
	t.store = newTreeStore(blockCount)
	t.rootToHeight = make(map[chainhash.Hash]int32, blockCount)
	t.hashToHeight = make(map[chainhash.Hash]int32, blockCount)
}

// AddBlockWithoutRebuild adds block as latest leaf, increases height and weight, but without tree rebuild.
//
// IMPORTANT! This function is not safe!
//
// AddBlockWithoutRebuild  should be used only for quick block adding.
//
// Quick Block Adding must be done like this:
//
//  tree := NewTree()
//  tree.PreAllocateTree(n)
//  for _, block := range blocks {
//     tree.AddBlockWithoutRebuild(....)
//  }
//  err := tree.RebuildTreeAndAssert()
func (t *BlocksMMRTree) AddBlockWithoutRebuild(hash, actualMMR chainhash.Hash, height int32, difficulty *big.Int) {
	node := &TreeNode{
		Hash:   hash,
		Weight: difficulty,
		Height: height,
	}

	t.hashToHeight[hash] = height
	t.rootToHeight[actualMMR] = height
	t.heightToRoot[height] = actualMMR

	t.store.appendLeaf(int(height), node)

	t.rootHash = actualMMR
	t.lastNode = node
	t.nextHeight = height + 1
	t.chainWeight = t.chainWeight.Add(t.chainWeight, difficulty)
}

// RebuildTreeAndAssert just rebuild the whole tree and checks is root match with actual.
func (t *BlocksMMRTree) RebuildTreeAndAssert() error {
	t.Lock()

	root := t.rebuildTree(t.lastNode, t.lastNode.Height)
	if !t.rootHash.IsEqual(&root) {
		t.Unlock()
		h1 := t.rootToHeight[t.rootHash]
		h2 := t.rootToHeight[root]
		return fmt.Errorf("mmr_root(%s, %d) of tree mismatches with calculated root(%s, %d); %d",
			t.rootHash, h1, root, h2, t.lastNode.Height)
	}

	t.Unlock()
	return nil
}

// SetBlock sets provided block with <hash, height> as latest.
// If block height is not latest, then reset tree to height - 1 and add AddBLock.
func (t *BlocksMMRTree) SetBlock(hash chainhash.Hash, difficulty *big.Int, height int32) {
	t.Lock()

	if height < t.nextHeight {
		t.rmBlock(height)
	}

	t.appendBlock(hash, difficulty)
}

// ResetRootTo sets provided block with <hash, height> as latest and drops all blocks after this.
func (t *BlocksMMRTree) ResetRootTo(hash chainhash.Hash, height int32) {
	t.Lock()
	t.resetRootTo(hash, height)
	t.Unlock()
}

func (t *BlocksMMRTree) resetRootTo(hash chainhash.Hash, height int32) {
	_, found := t.hashToHeight[hash]
	if !found {
		return
	}
	t.rmBlock(height + 1)
}

// RmBlock drops all block from latest to (including) provided block with <hash, height>.
func (t *BlocksMMRTree) RmBlock(hash chainhash.Hash, height int32) {
	t.Lock()
	t.rmBlock(height)
	t.Unlock()
}

func (t *BlocksMMRTree) rmBlock(height int32) {
	treeNode, _ := t.store.getNode(0, int(height))
	if treeNode == nil {
		return
	}
	droppedNodes := t.store.dropNodeFromTree(0, int(height))

	for _, treeNode := range droppedNodes {
		if treeNode == nil {
			continue
		}
		height, ok := t.hashToHeight[treeNode.Hash]
		if ok {
			t.chainWeight = t.chainWeight.Sub(t.chainWeight, treeNode.Weight)
			delete(t.heightToRoot, height)
			delete(t.hashToHeight, treeNode.Hash)
		}
		_, ok = t.rootToHeight[treeNode.Hash]
		if ok {
			delete(t.rootToHeight, treeNode.Hash)
		}
	}

	t.nextHeight = height
	if t.nextHeight == 0 {
		t.lastNode = &TreeNode{}
		t.rootHash = chainhash.ZeroHash
		return
	}

	t.lastNode, _ = t.store.getNode(0, int(t.nextHeight-1))
	t.rootHash = t.heightToRoot[t.nextHeight-1]
}

func (t *BlocksMMRTree) Current() *TreeNode {
	t.RLock()
	node := t.lastNode
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) CurrenWeight() *big.Int {
	t.RLock()
	node := t.chainWeight
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) Parent(height int32) *TreeNode {
	t.RLock()
	node, _ := t.store.getNode(0, int(height-1))
	if node == nil {
		node = &TreeNode{}
	}
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) Block(height int32) *TreeNode {
	t.RLock()
	node, _ := t.store.getNode(0, int(height))
	if node == nil {
		node = &TreeNode{}
	}
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) CurrentRoot() chainhash.Hash {
	return t.rootHash
}

func (t *BlocksMMRTree) RootForHeight(height int32) chainhash.Hash {
	t.RLock()
	hash := t.heightToRoot[height]
	t.RUnlock()
	return hash
}

func (t *BlocksMMRTree) LeafByHash(blockHash chainhash.Hash) (*TreeNode, bool) {
	t.RLock()
	var node *TreeNode
	height, found := t.hashToHeight[blockHash]
	if found {
		node, found = t.store.getNode(0, int(height))
	}
	t.RUnlock()
	return node, found
}

func (t *BlocksMMRTree) ActualRootForLeafByHash(blockHash chainhash.Hash) (chainhash.Hash, bool) {
	t.RLock()
	var actualMMRRoot chainhash.Hash

	height, found := t.hashToHeight[blockHash]
	if found {
		actualMMRRoot, found = t.heightToRoot[height]
	}

	t.RUnlock()
	return actualMMRRoot, found
}
func (t *BlocksMMRTree) LookupNodeByRoot(mmrRoot chainhash.Hash) (*TreeNode, bool) {
	t.RLock()
	node := &TreeNode{}
	height, found := t.rootToHeight[mmrRoot]
	if found {
		node, found = t.store.getNode(0, int(height))
	}

	t.RUnlock()
	return node, found
}

func (t *BlocksMMRTree) rebuildTree(node *TreeNode, height int32) (rootHash chainhash.Hash) {
	t.store.appendLeaf(int(height), node)
	root := t.store.calcRoot()
	return root.Hash
}
