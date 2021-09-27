// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

import (
	"errors"
	"fmt"
)

type blockDb struct {
	nodes  map[uint64]*BlockData
	blocks map[uint64]*BlockData
}

func MemoryDb() (res *blockDb) {
	res = &blockDb{
		nodes:  make(map[uint64]*BlockData),
		blocks: make(map[uint64]*BlockData),
	}
	return
}

func (d *blockDb) Nodes() (res []uint64, err error) {
	for index := range d.nodes {
		res = append(res, index)
	}
	return
}

func (d *blockDb) Blocks() (res []uint64, err error) {
	for index := range d.blocks {
		res = append(res, index)
	}
	return
}

func (d *blockDb) SetNode(index uint64, data *BlockData) error {
	d.nodes[index] = data
	return nil
}

func (d *blockDb) SetBlock(index uint64, data *BlockData) error {
	d.blocks[index] = data
	return nil
}

func (d *blockDb) GetNode(index uint64) (res *BlockData, err error) {
	var ok bool
	res, ok = d.nodes[index]
	if !ok {
		err = errors.New("node not exist")
	}
	return
}

func (d *blockDb) GetBlock(index uint64) (res *BlockData, err error) {
	var ok bool
	res, ok = d.blocks[index]
	if !ok {
		err = errors.New("node not exist")
	}
	return
}

func (d *blockDb) Debug() {
	fmt.Println("Debug:")
	for i, v := range d.blocks {
		fmt.Printf("\tBlock: %d [%d:%x]\n", i, v.Weight, v.Hash)
	}

	for i, v := range d.nodes {
		fmt.Printf("\tNodes: %d [%d:%x]\n", i, v.Weight, v.Hash)
	}
}
