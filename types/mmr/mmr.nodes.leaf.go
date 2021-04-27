// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

type leafIndex uint64

func LeafIndex(value uint64) (res *leafIndex) {
	v := leafIndex(value)
	return &v
}

func (x *leafIndex) GetLeftBranch() IBlockIndex {
	value := uint64(*x)
	if value&1 == 0 && value > 1 {
		return NodeIndex(uint64(*x) - 1)
	}
	return nil
}

func (x *leafIndex) GetSibling() IBlockIndex {
	value := uint64(*x)
	if value&1 == 1 {
		return LeafIndex(value - 1)
	} else {
		return LeafIndex(value + 1)
	}
}

func (x *leafIndex) RightUp() IBlockIndex {
	value := x.Index()
	if value&1 == 1 {
		return NodeIndex(value)
	}
	return nil
}

func (x *leafIndex) GetTop() IBlockIndex {
	value := uint64(*x)
	if value&1 == 0 {
		return LeafIndex(value)
	}
	return NodeIndex(value).GetTop()
}

func (x *leafIndex) Index() uint64 {
	return uint64(*x)
}

// GetPeaks calculates Peaks
// Algorithm:
//  1. Get to from current. Take it.
//  2. Go to the left branch.
//     - if no any left brnaches - return
//     - go to 1
func (x *leafIndex) GetPeaks() (res []IBlockIndex) {
	var peak IBlockIndex = x
	for {
		peak = peak.GetTop()
		res = append(res, peak)
		if peak = peak.GetLeftBranch(); peak == nil {
			return
		}
	}
}

// GetHeight return leaf height
// Leaf is always on the Zero height
func (x *leafIndex) GetHeight() uint64 {
	return 0
}

func (x *leafIndex) IsRight() bool {
	return x.Index()&1 == 1
}

func (x *leafIndex) SetValue(mmr *mmr, data *BlockData) {
	mmr.db.SetBlock(x.Index(), data)
}

func (x *leafIndex) Value(mmr *mmr) (*BlockData, bool) {
	return mmr.db.GetBlock(x.Index())
}

func (x *leafIndex) AppendValue(mmr *mmr, data *BlockData) {
	mmr.db.SetBlock(x.Index(), data)
	var node IBlockIndex = x
	for node.IsRight() {
		sibling := node.GetSibling()
		if parent := node.RightUp(); parent != nil {
			leftData, _ := sibling.Value(mmr)
			aggregated := mmr.aggregate(leftData, data)
			parent.SetValue(mmr, aggregated)
			data = aggregated
			node = parent
			continue
		}
		return
	}
}
