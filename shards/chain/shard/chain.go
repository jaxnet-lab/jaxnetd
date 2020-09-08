package shard

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
	"io"
	"time"
)

const (

	// MaxBlockHeaderPayload is the maximum number of bytes a block header can be.
	// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes +
	// PrevBlock and MerkleRoot hashes.
	maxBlockHeaderPayload = 16 + (chainhash.HashSize * 2)
)

//
type shardChain struct {
}

func (c *shardChain) NewBlockHeader(version int32, prevHash, merkleRootHash chainhash.Hash, mmr chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) chain.BlockHeader {
	panic("implement me")
}

func (c *shardChain) NewNode(blockHeader chain.BlockHeader, parent chain.IBlockNode) chain.IBlockNode {
	panic("implement me")
}

func (c *shardChain) Params() *chaincfg.Params {
	panic("implement me")
}

//
func Chain() chain.IChain {
	return &shardChain{}
}

func (c *shardChain) NewHeader() chain.BlockHeader {
	return &header{}
}

func (c *shardChain) IsBeacon() bool {
	return false
}

func (c *shardChain) ShardID() int32 {
	return 0
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

func (c *shardChain) MaxBlockHeaderPayload() int {
	return maxBlockHeaderPayload
}

func (c *shardChain) BlockHeaderOverhead() int {
	return maxBlockHeaderPayload + encoder.MaxVarIntPayload
}
