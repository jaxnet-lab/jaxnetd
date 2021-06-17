// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

import "math/big"

type BlockData struct {
	Weight *big.Int
	Hash   Hash
}

// Digest calculates Block Digest
func (d *BlockData) Digest(hasher Hasher) (result Hash) {
	h := hasher()
	weightData := d.Weight.Bytes()
	h.Write(weightData[:])
	h.Write(d.Hash[:])
	copy(result[:], h.Sum([]byte{}))
	return
}

// IBlockIndex
//   MMR Blocks navigation abstraction
//   Might be two types of MMR block
//    - Leafs. It's a bottom layer, representing block data
//    - Nodes. It's a Mountain nodes data, representing aggregation of bottom layers
type IBlockIndex interface {
	// GetLeftBranch returns left hand branch from current
	GetLeftBranch() IBlockIndex

	// GetTop returns ...
	GetTop() IBlockIndex

	// RightUp returns upper block navigating by right mountain side
	// If current block is left side - return nli
	RightUp() IBlockIndex

	// IsRight checks if current block is right side sibling
	IsRight() bool

	// GetSibling returns sibling
	// If current brunch is left - return right sibling
	// If current brunch is right - return left sibling
	GetSibling() IBlockIndex

	// GetHeight returns height of this block
	GetHeight() uint64

	// Index returns index of the block
	// if it's a leaf - returns leaf index
	// if it's a node - returns node index
	Index() uint64

	// Value returns Block data
	Value(mmr *ShardsMergedMiningTree) (*BlockData, bool)

	// SetValue Set block value
	SetValue(mmr *ShardsMergedMiningTree, data *BlockData) error
}

func getHeight(value uint64) (height uint64) {
	for {
		if value == 0 || value&1 == 1 {
			return
		}
		value = value >> 1
		height++
	}
}
