package shard

import (
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincore"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
)

const (
	// MaxBlockHeaderPayload is the maximum number of bytes a block header can be.
	// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes +
	// PrevBlock and MerkleRoot hashes.
	maxBlockHeaderPayload = 16 + (chainhash.HashSize * 2)
)

func Chain(shardID uint32, params *chaincore.Params, genesis *wire.MsgBlock, gHeight int32) chain.IChain {
	shard := &shardChain{
		shardID:      shardID,
		startVersion: genesis.Header.Version(),
	}

	hash := genesis.BlockHash()
	clone := params.ShardGenesis(shardID, gHeight, &hash)
	clone.GenesisHash = &hash
	shard.chainParams = clone

	// todo(mike)
	return shard
}

type shardChain struct {
	shardID      uint32
	startVersion chain.BVersion
	chainParams  *chaincore.Params
}

func (c *shardChain) NewBlockHeader(version int32, prevHash, merkleRootHash chainhash.Hash,
	mmr chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) chain.BlockHeader {
	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &header{
		prevBlock:  prevHash,
		merkleRoot: merkleRootHash,
		timestamp:  timestamp, // time.Unix(time.Now().Unix(), 0),
		bits:       bits,
		nonce:      nonce,
	}
}

func (c *shardChain) NewNode(blockHeader chain.BlockHeader, parent chain.IBlockNode) chain.IBlockNode {
	return BlockNode(blockHeader, parent)
}

func (c *shardChain) Params() *chaincore.Params {
	// todo(mike) [chaincfg] change me
	return c.chainParams
}

func (c *shardChain) NewHeader() chain.BlockHeader {
	return &header{}
}

func (c *shardChain) IsBeacon() bool {
	return false
}

func (c *shardChain) ShardID() int32 {
	return int32(c.shardID)
}

func (c *shardChain) GenesisBlock() interface{} {
	return &wire.MsgBlock{
		Header: NewBlockHeader(1, chainhash.Hash{}, genesisMerkleRoot,
			chainhash.Hash{}, time.Unix(0x495fab29, 0), 0x1d00ffff, 0x7c2bac1d),
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
