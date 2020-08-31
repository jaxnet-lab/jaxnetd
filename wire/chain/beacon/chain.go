package beacon

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/wire/chain"
	"gitlab.com/jaxnet/core/shard.core.git/wire/encoder"
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

//
func Chain() chain.IChain {
	return &shardChain{}
}

func (c *shardChain) NewHeader() chain.BlockHeader {
	return &header{}
}

func (c *shardChain) MaxBlockHeaderPayload() int {
	return maxBlockHeaderPayload
}

func (c *shardChain) BlockHeaderOverhead() int {
	return maxBlockHeaderPayload + encoder.MaxVarIntPayload
}
