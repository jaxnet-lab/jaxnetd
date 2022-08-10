/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_treeStore_allocate(t *testing.T) {
	tests := []struct {
		name     string
		baseSize int
		treeSize []int
	}{
		{name: "1->2", baseSize: 1, treeSize: []int{2, 1}},
		{name: "2->3", baseSize: 2, treeSize: []int{4, 2, 1}},
		{name: "3->4", baseSize: 3, treeSize: []int{8, 4, 2, 1}},
		{name: "4->5", baseSize: 4, treeSize: []int{8, 4, 2, 1}},
		{name: "5->6", baseSize: 5, treeSize: []int{16, 8, 4, 2, 1}},
		{name: "6->7", baseSize: 6, treeSize: []int{16, 8, 4, 2, 1}},
		{name: "7->8", baseSize: 7, treeSize: []int{16, 8, 4, 2, 1}},
		{name: "8->9", baseSize: 8, treeSize: []int{16, 8, 4, 2, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mTree := newTreeStore(tt.baseSize)
			mTree.allocate()
			if !assert.Equal(t, len(tt.treeSize), len(mTree.nodes)) {
				t.FailNow()
			}
			for level, size := range tt.treeSize {
				if !assert.Equal(t, size, len(mTree.nodes[level])) {
					t.FailNow()
				}
			}
		})
	}
}

func Test_treeStore_dropNodeFromTree(t *testing.T) {
	blockCount := int32(8)
	nodes := make([]*TreeNode, blockCount)

	for i := int32(0); i < blockCount; i++ {
		nodes[i] = &TreeNode{
			Hash: hash("leaf_" + strconv.Itoa(int(i))), Weight: newInt(int64(42_320 + i)),
			Height: i,
		}
	}
	tree := newTreeWithLeaves(nodes)
	tree.calcRoot()

	tree.dropNodeFromTree(0, 8)
	tree.dropNodeFromTree(0, 4)
}

var top *TreeNode

func Benchmark_calcMMRRoot(b *testing.B) {
	blockCount := int32(1024)
	nextPoT := nextPowerOfTwo(blockCount + 1)
	arraySize := uint64(nextPoT*2 - 1)

	nodes := make([]*TreeNode, arraySize)
	for i := int32(0); i < blockCount; i++ {
		nodes[heightToID(int32(i))] = &TreeNode{
			Hash: hash("leaf_" + strconv.Itoa(int(i))), Weight: newInt(int64(42_320 + i)),
			Height: int32(i),
		}
	}

	var rootLeaf *TreeNode
	for i := 0; i < b.N; i++ {
		rootLeaf = calcRootForBlockNodes(nodes)
		//	cpu: Intel(R) Core(TM) i7-9700K CPU @ 3.60GHz
		//	Benchmark_calcMMRRoot
		//	Benchmark_calcMMRRoot-8   	    2316	    780959 ns/op
		//	Benchmark_calcMMRRoot-8   	    2124	    531647 ns/op
		//	Benchmark_calcMMRRoot-8   	    2541	    598547 ns/op

	}

	top = rootLeaf
}

func Benchmark_calcMMRRootNoRecursion(b *testing.B) {
	blockCount := int32(1024)
	blocks := make([]*TreeNode, blockCount)
	for i := int32(0); i < blockCount; i++ {
		blocks[i] = &TreeNode{
			Hash:   hash("leaf_" + strconv.Itoa(int(i))),
			Weight: newInt(int64(42_320 + i)),
			Height: i,
			final:  true,
		}
	}

	nodes := make([]*TreeNode, len(blocks))
	//var tree = newTreeStore(len(blocks))

	var rootLeaf *TreeNode
	for i := 0; i < b.N; i++ {
		tree := newTreeWithLeaves(nodes)
		rootLeaf = tree.calcRoot()

		//	cpu: Intel(R) Core(TM) i7-9700K CPU @ 3.60GHz
		//	Benchmark_calcMMRRootNoRecursion
		//	Benchmark_calcMMRRootNoRecursion-8   	    1180	   1426903 ns/op X
		//	Benchmark_calcMMRRootNoRecursion-8   	    2218	    764648 ns/op X
		//  Benchmark_calcMMRRootNoRecursion-8   	    1119	   1368500 ns/op X
		//  Benchmark_calcMMRRootNoRecursion-8   	  321481	      3613 ns/op
		//  Benchmark_calcMMRRootNoRecursion-8   	  143278	     16297 ns/op -- create new tree for each round
	}

	top = rootLeaf
}

//cpu: Intel(R) Core(TM) i7-9700K CPU @ 3.60GHz
//	Benchmark_Recursion-8   	    2541	    598547 ns/op
//  Benchmark_NoRecursion-8   	  321481	      3613 ns/op

func Test_calcMMRRootNoRecursion(t *testing.T) {
	testSet := []int32{
		1,
		2,
		3,
		4,
		5,
		6,
		7,
		8,
		9,
		10,
		11,
		12,
		13,
		14,
		126,
		1024,
		9323,
	}

	for bi, blockCount := range testSet {
		t.Run(fmt.Sprintf("%d-blocks-count(%d)", bi, blockCount), func(t *testing.T) {

			nextPoT := nextPowerOfTwo(int32(blockCount + 1))
			arraySize := uint64(nextPoT*2 - 1)
			blocks := make([]*TreeNode, blockCount)
			for i := int32(0); i < blockCount; i++ {
				blocks[i] = &TreeNode{
					Hash:   hash("leaf_" + strconv.Itoa(int(i))),
					Weight: newInt(int64(42_320 + i)),
					final:  true,
					Height: int32(i),
				}
			}

			nodes := make([]*TreeNode, arraySize)
			nodesV2 := make([]*TreeNode, len(blocks))

			for i := range blocks {
				nodes[heightToID(int32(i))] = blocks[i]
				nodesV2[i] = blocks[i].Clone()
			}

			rootLeaf := calcRootForBlockNodes(nodes)

			var tree = newTreeWithLeaves(blocks)
			rootLeafV2 := tree.calcRoot()
			assert.Equal(t, rootLeaf.Hash.String(), rootLeafV2.Hash.String())
		})
	}
}
