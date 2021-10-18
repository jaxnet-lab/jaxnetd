/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"encoding/binary"
	"math"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const ValueSize = 40

type Value [ValueSize]byte

func (v Value) Block() (b Leaf) {
	copy(b.Hash[:], v[:32])
	b.Weight = binary.LittleEndian.Uint64(v[32:])
	return b
}

type Leaf struct {
	Hash   chainhash.Hash
	Weight uint64

	v      Value
	filled bool
}

func (b *Leaf) Value() Value {
	if b.filled {
		return b.v
	}

	copy(b.v[:32], b.Hash[:])

	binary.LittleEndian.PutUint64(b.v[32:], b.Weight)
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

func HashMerkleBranches(left, right *Leaf) *Leaf {
	var data [80]byte
	lv := left.Value()
	rv := right.Value()

	copy(data[:ValueSize], lv[:])
	copy(data[ValueSize:], rv[:])

	return &Leaf{
		Hash:   chainhash.HashH(data[:]),
		Weight: left.Weight + right.Weight,
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
