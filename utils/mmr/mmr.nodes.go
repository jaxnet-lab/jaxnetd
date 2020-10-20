// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package mmr

import "math/big"

type BlockData struct {
	Weight *big.Int
	Hash   Hash
}

//Calculate Block Digest
func (d *BlockData) Digest(hasher Hasher) (result Hash) {
	h := hasher()
	weightData := d.Weight.Bytes()
	h.Write(weightData[:])
	h.Write(d.Hash[:])
	copy(result[:], h.Sum([]byte{}))
	return
}

/*
  MMR Blocks navigation abstraction
  Might be two types of MMR block
   - Leafs. It's a bottom layer, representing block data
   - Nodes. It's a Mountain nodes data, representing aggregation of bottom layers
*/
type IBlockIndex interface {
	//Returns left hand branch from current
	GetLeftBranch() IBlockIndex
	//Get Peak Block from current
	GetTop() IBlockIndex

	//Get upper block navigating by right mountain side
	//If current block is left side - return nli
	RightUp() IBlockIndex

	//Check if current block is right side sibling
	IsRight() bool

	//Returns sibling
	// If current brunch is left - return right sibling
	// If current brunch is right - return left sibling
	GetSibling() IBlockIndex

	//Get height of this block
	GetHeight() uint64

	//Get Index of the block
	// if it's a leaf - returns leaf index
	// if it's a node - returns node index
	Index() uint64

	//Get Block data
	Value(mmr *mmr) (*BlockData, bool)

	//Set block value
	SetValue(mmr *mmr, data *BlockData)
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
