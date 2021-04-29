// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

type IStore interface {
	GetNode(index uint64) (res *BlockData, ok bool)
	SetNode(index uint64, data *BlockData)
	GetBlock(index uint64) (res *BlockData, ok bool)
	SetBlock(index uint64, data *BlockData)
	Nodes() (res []uint64)
	Blocks() (res []uint64)
	Debug()
}
