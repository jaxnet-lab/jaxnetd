/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"sync"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type BlocksMMRTree interface {
	Current() *BlockNode
	Parent(height int32) *BlockNode
	Block(height int32) *BlockNode
	CurrentRoot() chainhash.Hash
	RootForHeight(height int32) chainhash.Hash

	LookupNodeByRoot(chainhash.Hash) *BlockNode

	AddBlock(hash chainhash.Hash, difficulty uint64, height int32)
	RmBlock(hash chainhash.Hash, height int32)
}

type BlockNode struct {
	Block
	// ActualRoot is a root of the MMR Tree when this node was latest
	ActualRoot chainhash.Hash
	// PrevNode hash of previous block
	PrevNode *BlockNode
	// ID of node in the MMR Tree
	ID     uint32
	height int32
}

type Tree struct {
	sync.RWMutex

	nodeCount   uint32
	chainWeight uint64

	// map of nodes and their index at zero layer of tree
	nodes map[uint32]*BlockNode
	// map of nodes and actual tree roots, when the node was added
	mountainTops map[chainhash.Hash]uint32
	heightToID   map[int32]uint32

	rootHash chainhash.Hash
	lastNode *BlockNode
}

func NewTree() *Tree {
	return &Tree{
		nodes:        make(map[uint32]*BlockNode, 4096),
		mountainTops: make(map[chainhash.Hash]uint32, 4096),
		heightToID:   make(map[int32]uint32, 4096),
		lastNode:     &BlockNode{},
	}
}

func (t *Tree) AddBlock(hash chainhash.Hash, difficulty uint64, height int32) {
	t.Lock()
	node := &BlockNode{
		Block:    Block{Hash: hash, Weight: difficulty},
		height:   height,
		ID:       t.nodeCount,
		PrevNode: t.lastNode,
	}

	t.nodes[t.nodeCount] = node
	t.heightToID[height] = t.nodeCount

	t.nodeCount += 1
	t.chainWeight += difficulty

	t.rootHash = t.rebuildTree(0, t.nodeCount)

	t.nodes[node.ID].ActualRoot = t.rootHash
	t.mountainTops[t.rootHash] = node.ID
	t.lastNode = node
	t.Unlock()
}

func (t *Tree) RmBlock(hash chainhash.Hash, height int32) {
	t.Lock()
	id := t.heightToID[height]
	if id == 0 {
		t.Unlock()
		return
	}

	node := t.nodes[id]
	if !node.Hash.IsEqual(&hash) {
		t.Unlock()
		// todo: really strange situation
		return
	}

	t.chainWeight -= node.Weight

	t.nodes[id] = nil
	t.mountainTops[t.rootHash] = 0
	t.nodeCount -= 1

	t.lastNode = node.PrevNode
	t.rootHash = node.PrevNode.ActualRoot

	t.Unlock()
}

func (t *Tree) Current() *BlockNode {
	t.RLock()
	node := t.lastNode
	t.RUnlock()
	return node
}

func (t *Tree) Parent(height int32) *BlockNode {
	t.RLock()
	id := t.heightToID[height]
	if id == 0 {
		t.RUnlock()
		return nil
	}
	node := t.nodes[id-1]
	if node == nil {
		node = &BlockNode{}
	}
	t.RUnlock()
	return node
}

func (t *Tree) Block(height int32) *BlockNode {
	t.RLock()
	id, ok := t.heightToID[height]
	if !ok {
		t.RUnlock()
		return nil
	}

	node := t.nodes[id]
	if node == nil {
		node = &BlockNode{}
	}
	t.RUnlock()
	return node
}

func (t *Tree) CurrentRoot() chainhash.Hash {
	return t.rootHash
}

func (t *Tree) RootForHeight(height int32) chainhash.Hash {
	t.RLock()

	id, ok := t.heightToID[height]
	if !ok {
		t.RUnlock()
		return chainhash.ZeroHash
	}
	hash := t.nodes[id].ActualRoot
	t.RUnlock()
	return hash
}

func (t *Tree) LookupNodeByRoot(hash chainhash.Hash) *BlockNode {
	t.RLock()
	bNode := t.mountainTops[hash]
	node := t.nodes[bNode]
	if node == nil {
		node = &BlockNode{}
	}
	t.RUnlock()
	return node
}

func (t *Tree) rebuildTree(startOffset, count uint32) (rootHash chainhash.Hash) {
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(count)
	arraySize := uint32(nextPoT*2 - 1)

	if count == 1 {
		rootHash = t.nodes[0].Hash
		return
	}

	// todo: add here last state to not recalculate all tree each time

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := uint32(nextPoT)
	for i := startOffset; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case t.nodes[i] == nil:
			// merkles[offset] = nil
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
	return
}

func hashMerkleBranches(left, right *BlockNode) *BlockNode {
	var data [80]byte
	lv := left.Value()
	rv := right.Value()

	copy(data[:ValueSize], lv[:])
	copy(data[ValueSize:], rv[:])

	return &BlockNode{
		Block: Block{
			Hash:   chainhash.HashH(data[:]),
			Weight: left.Weight + right.Weight,
		},
		height: 0,
	}
}
