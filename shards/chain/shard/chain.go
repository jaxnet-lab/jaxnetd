package shard

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
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
	shardId uint32
}

func (c *shardChain) NewBlockHeader(version int32, prevHash, merkleRootHash chainhash.Hash, mmr chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) chain.BlockHeader {
	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &header{
		prevBlock:       prevHash,
		merkleRoot:      merkleRootHash,
		timestamp:       timestamp, //time.Unix(time.Now().Unix(), 0),
	}
}

func (c *shardChain) NewNode(blockHeader chain.BlockHeader, parent chain.IBlockNode) chain.IBlockNode {
	return BlockNode(blockHeader, parent)
}

func (c *shardChain) Params() *chaincfg.Params {
	return &chaincfg.JaxNetParams
}

//
func Chain(shardId uint32) chain.IChain {
	return &shardChain{
		shardId: shardId,
	}
}

func (c *shardChain) NewHeader() chain.BlockHeader {
	return &header{}
}

func (c *shardChain) IsBeacon() bool {
	return false
}

func (c *shardChain) ShardID() int32 {
	return int32(c.shardId)
}

func (c *shardChain) GenesisBlock() interface{} {
	return &wire.MsgBlock{
		Header:       NewBlockHeader(1, chainhash.Hash{}, genesisMerkleRoot, chainhash.Hash{}, time.Unix(0x495fab29, 0), 0x1d00ffff, 0x7c2bac1d),
		Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
	}
}

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
