// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

import "encoding/binary"

type IStore interface {
	GetNode(index uint64) (res *BlockData, err error)
	SetNode(index uint64, data *BlockData) error
	GetBlock(index uint64) (res *BlockData, err error)
	SetBlock(index uint64, data *BlockData) error
	Nodes() (res []uint64, err error)
	Blocks() (res []uint64, err error)
	Debug()
}

type keyType []byte

const (
	blockKeyID = 0x01
	nodeKeyID  = 0x02
)

func getBlockIndexKey(index uint64) (res keyType) {
	res = make([]byte, 9)
	res[0] = blockKeyID
	binary.LittleEndian.PutUint64(res[1:9], index)
	return
}

func getNodeIndexKey(index uint64) (res keyType) {
	res = make([]byte, 9)
	res[0] = nodeKeyID
	binary.LittleEndian.PutUint64(res[1:9], index)
	return
}
