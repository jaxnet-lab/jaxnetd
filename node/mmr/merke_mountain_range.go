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

type Value []byte

func (v Value) Block() (b Leaf) {
	copy(b.Hash[:], v[:chainhash.HashSize])
	b.Weight = new(big.Int).SetBytes(v[chainhash.HashSize:])
	return b
}

type Leaf struct {
	Hash   chainhash.Hash
	Weight *big.Int

	v      Value
	filled bool
}

func (b *Leaf) Value() Value {
	if b.filled {
		return b.v
	}

	wBytes := b.Weight.Bytes()

	b.v = make([]byte, chainhash.HashSize+len(wBytes))
	copy(b.v[:chainhash.HashSize], b.Hash[:])
	copy(b.v[chainhash.HashSize:], wBytes)

	b.filled = true
	return b.v
}

func BuildMerkleTreeStore(blocks []Leaf) []*Leaf {
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(uint64(len(blocks)))
	arraySize := nextPoT*2 - 1
	merkles := make([]*Leaf, arraySize)

	// Create the base transaction hashes and populate the array with them.
	for i := range blocks {
		merkles[i] = &blocks[i]
	}

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

		// When there is no right child, the parent is equal the left child.
		case merkles[i+1] == nil:
			newItem := merkles[i]
			merkles[offset] = newItem

		// The normal case sets the parent node to the double sha256
		// of the concatenation of the left and right children.
		default:
			newItem := HashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newItem
		}
		offset++
	}

	return merkles
}

func calcRoot(nodes []*Leaf, keepPrevTops bool) *Leaf {
	switch len(nodes) {
	case 1:
		return nodes[0]
	case 3:
		if keepPrevTops && nodes[1] != nil {
			return nodes[1]
		}
		nodes[1] = HashMerkleBranches(nodes[0], nodes[2])
		return nodes[1]
	default:
		midPoint := len(nodes) / 2

		if keepPrevTops && nodes[midPoint] != nil {
			return nodes[midPoint]
		}

		leftBranchRoot := calcRoot(nodes[:midPoint], keepPrevTops)
		rightBranchRoot := calcRoot(nodes[midPoint+1:], keepPrevTops)
		nodes[midPoint] = HashMerkleBranches(leftBranchRoot, rightBranchRoot)
		return nodes[midPoint]
	}
}

func BuildMerkleTreeStoreNG(blocks []Leaf) (*Leaf, []*Leaf) {
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(uint64(len(blocks)))
	// arraySize := len(blocks) * 2
	arraySize := nextPoT*2 - 1
	merkles := make([]*Leaf, arraySize)

	for i := range blocks {
		merkles[heightToID(int32(i))] = &blocks[i]
	}

	if len(blocks) == 1 {
		return &blocks[0], merkles
	}

	root := calcRoot(merkles, false)
	return root, merkles
}

func HashMerkleBranches(left, right *Leaf) *Leaf {
	if left == nil {
		return nil
	}

	if right == nil {
		return &Leaf{Hash: left.Hash, Weight: left.Weight}
	}

	lv := left.Value()
	rv := right.Value()

	data := make([]byte, len(lv)+len(rv))

	copy(data[:len(lv)], lv[:])
	copy(data[len(rv):], rv[:])

	return &Leaf{
		Hash:   chainhash.HashH(data),
		Weight: new(big.Int).Add(left.Weight, right.Weight),
	}
}

// nextPowerOfTwo returns the next highest power of two from a given number if
// it is not already a power of two.  This is a helper function used during the
// calculation of a merkle tree.
func nextPowerOfTwo(n uint64) int {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return int(n)
	}

	// Figure out and return the next power of two.
	exponent := uint(math.Log2(float64(n))) + 1
	return 1 << exponent // 2^exponent
}