// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

import (
	"math"
)

type nodeIndex uint64

var index1 = uint64(1)

func NodeIndex(value uint64) (res *nodeIndex) {
	v := nodeIndex(value)
	return &v
}

func (x *nodeIndex) GetHeight() uint64 {
	return getHeight(uint64(*x))
}

func (x *nodeIndex) GetLeftBranch() IBlockIndex {
	pow := uint64(math.Pow(2, float64(x.GetHeight()+1)))
	value := x.Index()
	if value > pow {
		return NodeIndex(value - pow)
	}
	return nil
}

func (x *nodeIndex) GetSibling() IBlockIndex {
	value := x.Index()
	shift := index1 << (x.GetHeight() + 1)
	return NodeIndex(value ^ shift)
}

func (x *nodeIndex) IsRight() bool {
	shift := index1 << (x.GetHeight() + 1)
	return shift&x.Index() == shift
}

func (x *nodeIndex) RightUp() IBlockIndex {
	value := x.Index()

	shift := index1 << (x.GetHeight() + 1)
	if shift&value == shift {
		value ^= index1 << x.GetHeight()
		if value > 0 {
			return NodeIndex(value)
		}
	}
	return nil
}

func (x *nodeIndex) GetTop() IBlockIndex {
	shift := index1 << x.GetHeight()
	value := uint64(*x)
	result := value
	for value != 0 && value&shift == shift {
		result = value
		value = value ^ shift
		shift <<= 1
	}
	return NodeIndex(result)
}

func (x *nodeIndex) Index() uint64 {
	return uint64(*x)
}

func (x *nodeIndex) SetValue(mmr *mmr, data *BlockData) {
	mmr.db.SetNode(x.Index(), data)
}

func (x *nodeIndex) Value(mmr *mmr) (*BlockData, bool) {
	return mmr.db.GetNode(x.Index())
}
