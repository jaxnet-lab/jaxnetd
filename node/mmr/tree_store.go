/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import "math"

type treeStore struct {
	nodes [][]*TreeNode
}

// newTreeStore creates new MMR Tree store and allocates space based on number of leafs.
// leafs is a nodes from 0-level, they are representing blockchain blocks.
func newTreeStore(leafCount int) *treeStore {
	nextPoT := nextPowerOfTwo(int32(leafCount))
	levelCount := int(math.Log2(float64(nextPoT))) + 1

	tree := make([][]*TreeNode, levelCount)

	nNodes := nextPoT
	for i := 0; i < levelCount; i++ {
		tree[i] = make([]*TreeNode, nNodes)
		nNodes /= 2
	}

	return &treeStore{nodes: tree}
}

func newTreeWithLeaves(blocks []*TreeNode) *treeStore {
	mTree := newTreeStore(len(blocks))
	for i := range blocks {
		mTree.nodes[0][i] = blocks[i]
	}

	return mTree
}

func (mTree *treeStore) levelsCount() int         { return len(mTree.nodes) }
func (mTree *treeStore) lenOfLevel(level int) int { return len(mTree.nodes[level]) }

func (mTree *treeStore) appendLeaf(index int, node *TreeNode) {
	if index <= len(mTree.nodes[0])-1 {
		mTree.nodes[0][index] = node
		return
	}

	mTree.allocate()
	mTree.nodes[0][index] = node
}

// allocate add space for next 2^x leafs and all new roots.
// TODO: improve description
func (mTree *treeStore) allocate() {
	leafCount := len(mTree.nodes[0]) + 1
	nextPoT := nextPowerOfTwo(int32(leafCount))
	levelCount := int(math.Log2(float64(nextPoT))) + 1
	if len(mTree.nodes) == levelCount {
		return
	}

	currentN := len(mTree.nodes)
	mTree.nodes = append(mTree.nodes, make([][]*TreeNode, levelCount-currentN)...)

	nNodes := nextPoT
	for i := 0; i < levelCount; i++ {
		mTree.nodes[i] = append(mTree.nodes[i], make([]*TreeNode, nNodes-len(mTree.nodes[i]))...)
		nNodes /= 2
	}
}

func (mTree *treeStore) setNode(level, index int, node *TreeNode) {
	mTree.nodes[level][index] = node
}

func (mTree *treeStore) getNode(level, index int) (*TreeNode, bool) {
	if len(mTree.nodes) < level+1 {
		return nil, false
	}
	if len(mTree.nodes[level]) < index+1 {
		return nil, false
	}

	top := mTree.nodes[level][index]
	final := false
	if top != nil {
		final = top.final
	}

	return top, final
}

func (mTree *treeStore) dropNodeFromTree(level, index int) []*TreeNode {
	startIdx := index
	var droppedNodes []*TreeNode
	for l := level; l < len(mTree.nodes); l++ {
		for i := startIdx; i < len(mTree.nodes[l]); i++ {
			droppedNodes = append(droppedNodes, mTree.nodes[l][i])
			mTree.nodes[l][i] = nil
		}
		startIdx /= 2
	}

	return droppedNodes
}

func (mTree *treeStore) calcRoot() *TreeNode {
	leavesCount := mTree.lenOfLevel(0)
	switch leavesCount {
	case 1:
		return mTree.nodes[0][0]
	case 2:
		root := hashLeafs(mTree.nodes[0][0], mTree.nodes[0][1])
		root.final = true
		mTree.nodes[1][0] = root
		return root
	}

	var root *TreeNode

	// cycle across tree levels
	for level := 0; level < mTree.levelsCount()-1; level++ {
		hashID := 0

		nodeCount := mTree.lenOfLevel(level)

		for leftID := 0; leftID < nodeCount; leftID += 2 {
			if _, final := mTree.getNode(level+1, hashID); final {
				hashID++
				continue
			}

			left, lFinal := mTree.getNode(level, leftID)
			right, rFinal := mTree.getNode(level, leftID+1)

			if left == nil && right == nil {
				continue
			}

			root = hashLeafs(left, right)
			root.final = lFinal && rFinal
			mTree.setNode(level+1, hashID, root)
			hashID++
		}
	}

	return root
}
