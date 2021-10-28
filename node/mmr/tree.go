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

type TreeLeaf struct {
	Leaf

	Height     uint64
	ActualRoot chainhash.Hash // ActualRoot is a root of the MMR Tree when this node was latest
}

func (n *TreeLeaf) MarshalJSON() ([]byte, error) {
	type dto struct {
		BlockHash  string
		Weight     uint64
		Height     uint64
		ActualRoot string
	}

	d := dto{
		BlockHash:  n.Hash.String(),
		Weight:     n.Weight,
		Height:     n.Height,
		ActualRoot: n.ActualRoot.String(),
	}

	return json.Marshal(d)
}

func (n *TreeLeaf) Clone() *TreeLeaf {
	if n == nil {
		return nil
	}

	clone := *n
	return &clone
}

type BlocksMMRTree struct {
	sync.RWMutex

	// nextHeight stores number of TreeLeaf.
	// nextHeight - 1 is last height in chain.
	nextHeight  uint64
	chainWeight uint64
	lastNode    *TreeLeaf
	rootHash    chainhash.Hash

	// nodes is a representation of Merkle Mountain Range tree.
	// ID starts from 0.
	nodes []*TreeLeaf
	// mountainTops is an association of mmr_root and corresponding block_node,
	// that was last in the chain for this root
	mountainTops map[chainhash.Hash]uint64
	// hashToHeight is a map of hashes and their IDs, ID eq to height of block in chain.
	hashToHeight map[chainhash.Hash]uint64
}

func NewTree() *BlocksMMRTree {
	return &BlocksMMRTree{
		lastNode:     &TreeLeaf{},
		nodes:        make([]*TreeLeaf, 1),
		mountainTops: make(map[chainhash.Hash]uint64, 4096),
		hashToHeight: make(map[chainhash.Hash]uint64, 4096),
	}
}

func (t *BlocksMMRTree) MarshalJSON() ([]byte, error) {
	type dto struct {
		NodeCount    uint64
		ChainWeight  uint64
		Nodes        []*TreeLeaf
		MountainTops map[string]uint64
		HashToID     map[string]uint64
	}
	d := dto{
		NodeCount:    t.nextHeight,
		ChainWeight:  t.chainWeight,
		Nodes:        make([]*TreeLeaf, len(t.nodes)),
		MountainTops: make(map[string]uint64, len(t.mountainTops)),
		HashToID:     make(map[string]uint64, len(t.mountainTops)),
	}

	for u, node := range t.nodes {
		d.Nodes[u] = node.Clone()
	}
	for top, nodeID := range t.mountainTops {
		d.MountainTops[top.String()] = nodeID
	}

	for hash, nodeID := range t.hashToHeight {
		d.HashToID[hash.String()] = nodeID
	}

	return json.Marshal(d)
}
func (t *BlocksMMRTree) Fork() *BlocksMMRTree {
	t.RLock()
	newTree := &BlocksMMRTree{
		nextHeight:   t.nextHeight,
		chainWeight:  t.chainWeight,
		rootHash:     chainhash.Hash{},
		lastNode:     t.lastNode.Clone(),
		nodes:        make([]*TreeLeaf, len(t.nodes)),
		mountainTops: make(map[chainhash.Hash]uint64, len(t.mountainTops)),
		hashToHeight: make(map[chainhash.Hash]uint64, len(t.hashToHeight)),
	}

	for u, node := range t.nodes {
		newTree.nodes[u] = node.Clone()
	}
	for top, nodeID := range t.mountainTops {
		newTree.mountainTops[top] = nodeID
	}

	for hash, nodeID := range t.hashToHeight {
		newTree.hashToHeight[hash] = nodeID
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
	_, ok := t.hashToHeight[hash]
	if ok {
		return
	}

	node := &TreeLeaf{
		Leaf:   Leaf{Hash: hash, Weight: difficulty},
		Height: t.nextHeight,
	}

	t.hashToHeight[hash] = node.Height

	t.nextHeight += 1
	t.chainWeight += difficulty

	t.rootHash = t.rebuildTree(node, node.Height+1)

	t.nodes[heightToID(int32(node.Height))].ActualRoot = t.rootHash
	t.mountainTops[t.rootHash] = node.Height
	t.lastNode = node
}

// SetBlock sets provided block with <hash, height> as latest.
// If block height is not latest, then reset tree to height - 1 and add AddBLock.
func (t *BlocksMMRTree) SetBlock(hash chainhash.Hash, difficulty uint64, height int32) {
	t.Lock()

	if uint64(height) < t.nextHeight {
		node := t.nodes[heightToID(height)]
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
	_, found := t.hashToHeight[hash]
	if !found {
		return
	}

	if t.nextHeight < uint64(height+1) || int32(len(t.nodes)) < height+2 {
		return
	}

	node := t.nodes[heightToID(height+1)]
	if node == nil {
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
	id, found := t.hashToHeight[hash]
	if !found {
		return
	}

	if t.nextHeight < uint64(height) || int32(len(t.nodes)) < height+1 {
		return
	}

	node := t.nodes[heightToID(height)]
	if node == nil || !node.Hash.IsEqual(&hash) || id != uint64(height) {
		return
	}

	// remove nodes
	for i := heightToID(height); i < uint64(len(t.nodes)); i += 2 {
		leaf := t.nodes[i]
		if leaf == nil {
			continue
		}
		delete(t.hashToHeight, leaf.Hash)
		delete(t.mountainTops, leaf.ActualRoot)
		t.chainWeight -= leaf.Weight
		t.nodes[i] = nil
	}

	// remove tops
	for i := heightToID(height) - 1; i < uint64(len(t.nodes)); i += 2 {
		leaf := t.nodes[i]
		if leaf == nil {
			continue
		}

		delete(t.mountainTops, leaf.Hash)
		t.nodes[i] = nil
	}

	t.nextHeight = uint64(height)

	if t.nextHeight == 0 {
		t.lastNode = &TreeLeaf{}
		t.rootHash = chainhash.ZeroHash
		return
	}

	t.lastNode = t.nodes[heightToID(int32(t.nextHeight-1))]
	t.rootHash = t.lastNode.ActualRoot

}

func (t *BlocksMMRTree) Current() *TreeLeaf {
	t.RLock()
	node := t.lastNode
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) CurrenWeight() uint64 {
	t.RLock()
	node := t.chainWeight
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) Parent(height int32) *TreeLeaf {
	t.RLock()
	node := t.nodes[heightToID(height-1)]
	if node == nil {
		node = &TreeLeaf{}
	}
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) Block(height int32) *TreeLeaf {
	t.RLock()
	node := t.nodes[heightToID(height)]
	if node == nil {
		node = &TreeLeaf{}
	}
	t.RUnlock()
	return node
}

func (t *BlocksMMRTree) CurrentRoot() chainhash.Hash {
	return t.rootHash
}

func (t *BlocksMMRTree) RootForHeight(height int32) chainhash.Hash {
	t.RLock()
	hash := t.nodes[heightToID(height)].ActualRoot
	t.RUnlock()
	return hash
}

func (t *BlocksMMRTree) LeafByHash(blockHash chainhash.Hash) (*TreeLeaf, bool) {
	t.RLock()
	var node *TreeLeaf
	height, found := t.hashToHeight[blockHash]
	if found {
		node = t.nodes[heightToID(int32(height))]
	}
	t.RUnlock()
	return node, found
}

func (t *BlocksMMRTree) LookupNodeByRoot(mmrRoot chainhash.Hash) (*TreeLeaf, bool) {
	t.RLock()
	node := &TreeLeaf{}
	height, found := t.mountainTops[mmrRoot]
	if found {
		node = t.nodes[heightToID(int32(height))]
	}

	t.RUnlock()
	return node, found
}

func (t *BlocksMMRTree) rebuildTree(node *TreeLeaf, count uint64) (rootHash chainhash.Hash) {
	if count == 1 {
		t.nodes = []*TreeLeaf{node}
		rootHash = node.Hash
		return
	}

	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(count)
	arraySize := uint64(nextPoT*2 - 1)
	if len(t.nodes) < nextPoT {
		blockNodes := make([]*TreeLeaf, arraySize)
		for i := range t.nodes {
			blockNodes[i] = t.nodes[i]
		}
		t.nodes = blockNodes
	}

	t.nodes[heightToID(int32(node.Height))] = node

	rootID := len(t.nodes) / 2
	t.nodes[rootID] = nil
	rootHash = calcRootForBlockNodes(t.nodes).Hash
	return
}

func calcRootForBlockNodes(nodes []*TreeLeaf) *TreeLeaf {
	switch len(nodes) {
	case 1:
		return nodes[0]
	case 3:
		if nodes[1] != nil {
			return nodes[1]
		}
		top, final := hashMerkleBranches(nodes[0], nodes[2])
		if final {
			nodes[1] = top
		}

		return top
	default:
		midPoint := len(nodes) / 2

		leftBranchRoot := calcRootForBlockNodes(nodes[:midPoint])
		rightBranchRoot := calcRootForBlockNodes(nodes[midPoint+1:])
		top, final := hashMerkleBranches(leftBranchRoot, rightBranchRoot)
		if final {
			nodes[midPoint] = top
		}

		return top
	}
}

func hashMerkleBranches(left, right *TreeLeaf) (*TreeLeaf, bool) {
	if left == nil {
		return nil, false
	}

	if right == nil {
		return &TreeLeaf{Leaf: Leaf{Hash: left.Hash, Weight: left.Weight}}, false
	}

	var data [80]byte
	lv := left.Value()
	rv := right.Value()

	copy(data[:ValueSize], lv[:])
	copy(data[ValueSize:], rv[:])

	return &TreeLeaf{
		Leaf: Leaf{
			Hash:   chainhash.HashH(data[:]),
			Weight: left.Weight + right.Weight,
		},
	}, true
}

func heightToID(h int32) uint64 {
	return uint64(h * 2)
}
