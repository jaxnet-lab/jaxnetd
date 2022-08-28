/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chainhash

import (
	"math"
)

// BuildMerkleTreeStore creates a merkle tree from a slice of transactions,
// stores it using a linear array, and returns a slice of the backing array.  A
// linear array was chosen as opposed to an actual tree structure since it uses
// about half as much memory.  The following describes a merkle tree and how it
// is stored in a linear array.
//
// A merkle tree is a tree in which every non-leaf node is the Hash of its
// children nodes.  A diagram depicting how this works for bitcoin transactions
// where h(x) is a double sha256 follows:
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
//	[h1 h2 h3 h4 h12 h34 root]
//
// As the above shows, the merkle root is always the last element in the array.
//
// The number of inputs is not always a power of two which results in a
// balanced tree structure as above.  In that case, parent nodes with no
// children are also zero and parent nodes with only a single left node
// are calculated by concatenating the left node with itself before hashing.
// Since this function uses nodes that are pointers to the hashes, empty nodes
// will be nil.
//
// The additional bool parameter indicates if we are generating the merkle tree
// using witness transaction id's rather than regular transaction id's. This
// also presents an additional case wherein the wtxid of the coinbase transaction
// is the zeroHash.
func BuildMerkleTreeStore(txHashes []Hash) []*Hash {
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := NextPowerOfTwo(len(txHashes))
	arraySize := nextPoT*2 - 1
	merkles := make([]*Hash, arraySize)

	// Create the base transaction hashes and populate the array with them.
	for i := range txHashes {
		merkles[i] = &txHashes[i]
	}

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

		// When there is no right child, the parent is generated by
		// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := HashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

		// The normal case sets the parent node to the double sha256
		// of the concatenation of the left and right children.
		default:
			newHash := HashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}

func MerkleTreeRoot(txHashes []Hash) Hash {
	tree := BuildMerkleTreeStore(txHashes)
	return *tree[len(tree)-1]
}

func ValidateMerkleTreeRoot(txHashes []Hash, expectedRoot Hash) bool {
	tree := BuildMerkleTreeStore(txHashes)
	return tree[len(tree)-1].IsEqual(&expectedRoot)
}

// nolint: gocritic
func BuildCoinbaseMerkleTreeProof(txHashes []Hash) []Hash {
	merkleHashes := txHashes
	steps := make([]Hash, 0)
	PreL := []Hash{{}}
	StartL := 2
	Ll := len(merkleHashes)

	for Ll > 1 {
		steps = append(steps, merkleHashes[1])

		if Ll%2 != 0 {
			merkleHashes = append(merkleHashes, merkleHashes[len(merkleHashes)-1])
		}

		r := rangeSteps(StartL, Ll, 2)
		Ld := make([]Hash, len(r))

		for i := 0; i < len(r); i++ {
			Ld[i] = *HashMerkleBranches(&merkleHashes[r[i]], &merkleHashes[r[i]+1])
		}
		merkleHashes = append(PreL, Ld...)
		Ll = len(merkleHashes)
	}

	return steps
}

func CoinbaseMerkleTreeProofRoot(txHash Hash, proof []Hash) Hash {
	root := txHash
	for i := range proof {
		root = *HashMerkleBranches(&root, &proof[i])
	}

	return root
}

func ValidateCoinbaseMerkleTreeProof(txHash Hash, proof []Hash, expectedRoot Hash) bool {
	root := txHash
	for i := range proof {
		root = *HashMerkleBranches(&root, &proof[i])
	}

	return root.IsEqual(&expectedRoot)
}

// rangeSteps steps between [start, end)
func rangeSteps(start, stop, step int) []int {
	if (step > 0 && start >= stop) || (step < 0 && start <= stop) {
		return []int{}
	}

	result := make([]int, 0)
	i := start
	for {
		if step > 0 {
			if i < stop {
				result = append(result, i)
			} else {
				break
			}
		} else {
			if i > stop {
				result = append(result, i)
			} else {
				break
			}
		}
		i += step
	}

	return result
}

// HashMerkleBranches takes two hashes, treated as the left and right tree
// nodes, and returns the Hash of their concatenation.  This is a helper
// function used to aid in the generation of a merkle tree.
func HashMerkleBranches(left *Hash, right *Hash) *Hash {
	// Concatenate the left and right nodes.
	var hash [HashSize * 2]byte
	copy(hash[:HashSize], left[:])
	copy(hash[HashSize:], right[:])

	newHash := DoubleHashH(hash[:])
	return &newHash
}

// NextPowerOfTwo returns the next highest power of two from a given number if
// it is not already a power of two.  This is a helper function used during the
// calculation of a merkle tree.
func NextPowerOfTwo(n int) int {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return n
	}

	// Figure out and return the next power of two.
	exponent := uint(math.Log2(float64(n))) + 1
	return 1 << exponent // 2^exponent
}
