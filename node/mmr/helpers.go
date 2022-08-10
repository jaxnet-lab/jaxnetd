/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"math"
	"math/big"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

var newInt = big.NewInt
var newUint = func(x uint64) *big.Int { return new(big.Int).SetUint64(x) }

// nextPowerOfTwo returns the next highest power of two from a given number if
// it is not already a power of two.  This is a helper function used during the
// calculation of a merkle tree.
func nextPowerOfTwo(n int32) int {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return int(n)
	}

	// Figure out and return the next power of two.
	exponent := uint(math.Log2(float64(n))) + 1
	return 1 << exponent // 2^exponent
}

func hashNodes(left, right *TreeNode) (*TreeNode, bool) {
	if left == nil {
		return nil, false
	}

	if right == nil {
		return &TreeNode{Hash: left.Hash, Weight: left.Weight}, false
	}

	lv := left.Bytes()
	rv := right.Bytes()

	data := make([]byte, len(lv)+len(rv))

	copy(data[:len(lv)], lv[:])
	copy(data[len(rv):], rv[:])

	return &TreeNode{
		Hash:   chainhash.HashH(data),
		Weight: new(big.Int).Add(left.Weight, right.Weight),
	}, true
}

// TODO: delete after refactoring
func hashLeafs(left, right *TreeNode) *TreeNode {
	root, _ := hashNodes(left, right)
	return root
}

func heightToID(h int32) uint64 { return uint64(h * 2) }

func calcRootForBlockNodes(nodes []*TreeNode) *TreeNode {
	switch len(nodes) {
	case 1:
		return nodes[0]
	case 3:
		if nodes[1] != nil {
			return nodes[1]
		}
		top, final := hashNodes(nodes[0], nodes[2])
		if final {
			nodes[1] = top
		}

		return top
	default:
		midPoint := len(nodes) / 2

		leftBranchRoot := calcRootForBlockNodes(nodes[:midPoint])
		rightBranchRoot := calcRootForBlockNodes(nodes[midPoint+1:])
		top, final := hashNodes(leftBranchRoot, rightBranchRoot)
		if final {
			nodes[midPoint] = top
		}

		return top
	}
}
