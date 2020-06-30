package mmr

import (
	"fmt"
	"sync"
)

type blockDb struct {
	sync.RWMutex
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

func (d *blockDb) Nodes() (res []uint64) {
	for index := range d.nodes {
		res = append(res, index)
	}
	return
}

func (d *blockDb) Blocks() (res []uint64) {
	for index := range d.blocks {
		res = append(res, index)
	}
	return
}

func (d *blockDb) SetNode(index uint64, data *BlockData) {
	d.nodes[index] = data
}

func (d *blockDb) SetBlock(index uint64, data *BlockData) {
	d.blocks[index] = data
}

func (d *blockDb) GetNode(index uint64) (res *BlockData, ok bool) {
	res, ok = d.nodes[index]
	return
}

func (d *blockDb) GetBlock(index uint64) (res *BlockData, ok bool) {
	res, ok = d.blocks[index]
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
