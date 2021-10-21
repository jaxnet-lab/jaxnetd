/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"encoding/json"
	"sync"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type _BlocksMMRTree interface {
	Current() *BlockNode
	Parent(height int32) *BlockNode
	Block(height int32) *BlockNode
	CurrentRoot() chainhash.Hash
	RootForHeight(height int32) chainhash.Hash

	LookupNodeByRoot(chainhash.Hash) (*BlockNode, bool)

	AddBlock(hash chainhash.Hash, difficulty uint64)
	SetBlock(hash chainhash.Hash, difficulty uint64, height int32)
	RmBlock(hash chainhash.Hash, height int32)
	ResetRootTo(hash chainhash.Hash, height int32)
}

type BlockNode struct {
	Leaf

	ID         uint64         // ID of node in the MMR Tree, if ID < math.MaxInt32, ID == block height in main chain.
	PrevNodeID uint64         // PrevNodeID hash of previous block
	ActualRoot chainhash.Hash // ActualRoot is a root of the MMR Tree when this node was latest
}

func (n *BlockNode) MarshalJSON() ([]byte, error) {
	type dto struct {
		BlockHash  string
		Weight     uint64
		ID         uint64
		PrevNodeID uint64
		ActualRoot string
	}

	d := dto{
		BlockHash:  n.Hash.String(),
		Weight:     n.Weight,
		ID:         n.ID,
		PrevNodeID: n.PrevNodeID,
		ActualRoot: n.ActualRoot.String(),
	}

	return json.Marshal(d)
}

func (n *BlockNode) Clone() *BlockNode {
	clone := *n
	return &clone
}

type BlocksMMRTree struct {
	sync.RWMutex

	// nodeCount stores number of BlockNode.
	// nodeCount - 1 is last height in chain.
	nodeCount   uint64
	chainWeight uint64
	lastRootID  uint64
	lastNode    *BlockNode
	rootHash    chainhash.Hash

	// nodes is a representation of Merkle Mountain Range tree.
	// ID starts from 0.
	// If ID < math.MaxInt32, ID == block height in main chain.
	// If ID > math.MaxInt32, ID is a tree leaf or top.
	nodes map[uint64]*BlockNode
	// mountainTops is an association of mmr_root and corresponding block_node,
	// that was last in the chain for this root
	mountainTops map[chainhash.Hash]uint64
	// hashToID is a map of hashes and their IDs, ID eq to height of block in chain.
	hashToID map[chainhash.Hash]uint64
}

func NewTree() *BlocksMMRTree {
	return &BlocksMMRTree{
		lastNode:     &BlockNode{},
		nodes:        make(map[uint64]*BlockNode, 4096),
		mountainTops: make(map[chainhash.Hash]uint64, 4096),
		hashToID:     make(map[chainhash.Hash]uint64, 4096),
	}
}

func (t *BlocksMMRTree) MarshalJSON() ([]byte, error) {
	type dto struct {
		NodeCount    uint64
		ChainWeight  uint64
		LastRootID   uint64
		Nodes        map[uint64]*BlockNode
		MountainTops map[string]uint64
		HashToID     map[string]uint64
	}
	d := dto{
		NodeCount:    t.nodeCount,
		ChainWeight:  t.chainWeight,
		LastRootID:   t.lastRootID,
		Nodes:        make(map[uint64]*BlockNode, len(t.nodes)),
		MountainTops: make(map[string]uint64, len(t.mountainTops)),
		HashToID:     make(map[string]uint64, len(t.mountainTops)),
	}

	for u, node := range t.nodes {
		d.Nodes[u] = node.Clone()
	}
	for top, nodeID := range t.mountainTops {
		d.MountainTops[top.String()] = nodeID
	}

	for hash, nodeID := range t.hashToID {
		d.HashToID[hash.String()] = nodeID
	}

	return json.Marshal(d)
}
func (t *BlocksMMRTree) Fork() *BlocksMMRTree {
	t.RLock()
	newTree := &BlocksMMRTree{
		nodeCount:    t.nodeCount,
		chainWeight:  t.chainWeight,
		rootHash:     chainhash.Hash{},
		lastNode:     t.lastNode.Clone(),
		nodes:        make(map[uint64]*BlockNode, len(t.nodes)),
		mountainTops: make(map[chainhash.Hash]uint64, len(t.mountainTops)),
		hashToID:     make(map[chainhash.Hash]uint64, len(t.hashToID)),
	}

	for u, node := range t.nodes {
		newTree.nodes[u] = node.Clone()
	}
	for top, nodeID := range t.mountainTops {
		newTree.mountainTops[top] = nodeID
	}

	for hash, nodeID := range t.hashToID {
		newTree.hashToID[hash] = nodeID
	}

	t.RUnlock()
	return newTree
}

// AddBlock adds block as latest leaf, increases height and rebuild tree.
func (t *BlocksMMRTree) AddBlock(hash chainhash.Hash, difficulty uint64) {
	t.Lock()
	t.addBlock(hash, difficulty)
	t.Unlock()
}

func (t *BlocksMMRTree) addBlock(hash chainhash.Hash, difficulty uint64) {
	_, ok := t.hashToID[hash]
	if ok {
		return
	}

	node := &BlockNode{
		Leaf:       Leaf{Hash: hash, Weight: difficulty},
		ID:         t.nodeCount,
		PrevNodeID: t.lastNode.ID,
	}

	t.nodes[node.ID] = node
	t.hashToID[hash] = node.ID

	t.nodeCount += 1
	t.chainWeight += difficulty

	t.rootHash = t.rebuildTree(0, node.ID+1)

	t.nodes[node.ID].ActualRoot = t.rootHash
	t.mountainTops[t.rootHash] = node.ID
	t.lastNode = node
}

// SetBlock sets provided block with <hash, height> as latest.
// If block height is not latest, then reset tree to height - 1 and add AddBLock.
func (t *BlocksMMRTree) SetBlock(hash chainhash.Hash, difficulty uint64, height int32) {
	t.Lock()

	if uint64(height) < t.nodeCount {
		node := t.nodes[uint64(height)]
		t.rmBlock(node.Hash, height)
	}

	t.addBlock(hash, difficulty)
	return
}

// ResetRootTo sets provided block with <hash, height> as latest and drops all blocks after this.
func (t *BlocksMMRTree) ResetRootTo(hash chainhash.Hash, height int32) {
	t.Lock()
	t.resetRootTo(hash, height)
	t.Unlock()
}

func (t *BlocksMMRTree) resetRootTo(hash chainhash.Hash, height int32) {
	_, found := t.hashToID[hash]
	if !found {
		return
	}
	_, found = t.nodes[uint64(height)]
	if !found {
		return
	}

	node, found := t.nodes[uint64(height+1)]
	if !found {
		return
	}

	t.rmBlock(node.Hash, height+1)
}

// RmBlock drops all block from latest to (including) provided block with <hash, height>.
func (t *BlocksMMRTree) RmBlock(hash chainhash.Hash, height int32) {
	t.Lock()
	t.rmBlock(hash, height)
	t.Unlock()
}

func (t *BlocksMMRTree) rmBlock(hash chainhash.Hash, height int32) {

	id, found := t.hashToID[hash]
	if !found {
		return
	}

	node, found := t.nodes[uint64(height)]
	if !found || node == nil || !node.Hash.IsEqual(&hash) || id != uint64(height) {
		return
	}

	var deleted = 0
	for idToDrop := t.nodeCount - 1; idToDrop >= node.ID; idToDrop-- {
		node := t.nodes[idToDrop]
		t.nodeCount -= 1
		t.chainWeight -= node.Weight
		delete(t.hashToID, node.Hash)
		delete(t.mountainTops, node.ActualRoot)
		delete(t.nodes, idToDrop)
		deleted++
	}

	if t.nodeCount == 0 {
		return
	}

	t.lastNode = t.nodes[t.nodeCount-1]
	t.rootHash = t.lastNode.ActualRoot

}

func (t *BlocksMMRTree) Current() *BlockNode {
	t.RLock()
	node := t.lastNode
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) Parent(height int32) *BlockNode {
	t.RLock()
	node := t.nodes[uint64(height-1)]
	if node == nil {
		node = &BlockNode{}
	}
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) Block(height int32) *BlockNode {
	t.RLock()
	node := t.nodes[uint64(height)]
	if node == nil {
		node = &BlockNode{}
	}
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) CurrentRoot() chainhash.Hash {
	return t.rootHash
}

func (t *BlocksMMRTree) RootForHeight(height int32) chainhash.Hash {
	t.RLock()
	hash := t.nodes[uint64(height)].ActualRoot
	t.RUnlock()
	return hash
}

func (t *BlocksMMRTree) LookupNodeByRoot(hash chainhash.Hash) (*BlockNode, bool) {
	t.RLock()
	node := &BlockNode{}
	bNode, found := t.mountainTops[hash]

	if found {
		node, found = t.nodes[bNode]
	}

	t.RUnlock()
	return node, found
}

func (t *BlocksMMRTree) rebuildTree(startOffset, count uint64) (rootHash chainhash.Hash) {
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(count)
	arraySize := uint64(nextPoT*2 - 1)

	if count == 1 {
		rootHash = t.nodes[0].Hash
		return
	}

	// todo: add here last state to not recalculate all tree each time

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := uint64(nextPoT)
	for i := startOffset; i < arraySize+1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case t.nodes[i] == nil:
			// t.nodes[offset] = nil
			continue

		// When there is no right child, the parent is equal the left child.
		case t.nodes[i+1] == nil:
			newItem := t.nodes[i]
			t.nodes[offset] = newItem

		// The normal case sets the parent node to the double sha256
		// of the concatenation of the left and right children.
		default:
			newItem := hashMerkleBranches(t.nodes[i], t.nodes[i+1])
			newItem.ID = offset
			t.nodes[offset] = newItem
			rootHash = newItem.Hash
		}
		offset++
	}

	t.lastRootID = offset - 1
	return
}

func hashMerkleBranches(left, right *BlockNode) *BlockNode {
	var data [80]byte
	lv := left.Value()
	rv := right.Value()

	copy(data[:ValueSize], lv[:])
	copy(data[ValueSize:], rv[:])

	return &BlockNode{
		Leaf: Leaf{
			Hash:   chainhash.HashH(data[:]),
			Weight: left.Weight + right.Weight,
		},
	}
}
