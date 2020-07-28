package shard

import (
	"gitlab.com/jaxnet/core/shard.core.git/wire/chain"
	"io"
)

//
type shardChain struct {
}

//
func Chain() chain.IChain {
	return &shardChain{}
}

func (c *shardChain) NewHeader() chain.BlockHeader {
	return &header{}
}

//
//func (c *shardChain) BlockOne() {
//
//}
//
//func (c *shardChain) GenesisHash() chainhash.Hash {
//	return [32]byte{}
//}

func (c *shardChain) Read(r io.Reader) (chain.BlockHeader, error) {
	h := &header{}
	err := readBlockHeader(r, h)
	return h, err
}

func (c *shardChain) Write(w io.Writer, h chain.BlockHeader) error {
	header := h.(*header)
	return writeBlockHeader(w, header)
}
