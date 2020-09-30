package shard

import (
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
)

func Chain(shardID uint32, params *chaincfg.Params, beaconGenesisBlc *wire.MsgBlock, gHeight int32) chain.IChain {
	shard := &shardChain{
		beaconHeight: gHeight,
		shardID:      shardID,
		startVersion: beaconGenesisBlc.Header.Version(),
	}
	clone := params.ShardGenesis(shardID, nil)
	clone.GenesisBlock = chaincfg.GenesisBlockOpts{
		Version:   int32(beaconGenesisBlc.Header.Version()),
		Timestamp: beaconGenesisBlc.Header.Timestamp(),

		// todo(mike) ensure that this is correct
		PrevBlock:  chainhash.Hash{},
		MerkleRoot: chainhash.Hash{},
		Bits:       beaconGenesisBlc.Header.Bits(),
		Nonce:      beaconGenesisBlc.Header.Nonce(),
	}
	shard.chainParams = clone

	genesis := shard.GenesisBlock().(*wire.MsgBlock)
	hash := genesis.BlockHash()
	clone.GenesisHash = &hash

	return shard
}

type shardChain struct {
	shardID      uint32
	beaconHeight int32
	startVersion chain.BVersion
	chainParams  *chaincfg.Params
}

func (c *shardChain) NewBlockHeader(version chain.BVersion, prevHash, merkleRootHash chainhash.Hash,
	mmr chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) chain.BlockHeader {
	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return chain.NewShardBlockHeader(
		version,
		prevHash,
		merkleRootHash,
		mmr,
		timestamp,
		bits,
		nonce,
	)
}

func (c *shardChain) NewNode(blockHeader chain.BlockHeader, parent chain.IBlockNode) chain.IBlockNode {
	return chain.NewShardBlockNode(blockHeader, parent)
}

func (c *shardChain) Params() *chaincfg.Params {
	// todo(mike) [chaincfg] change me
	return c.chainParams
}

func (c *shardChain) NewHeader() chain.BlockHeader {
	return &chain.ShardHeader{}
}

func (c *shardChain) IsBeacon() bool {
	return false
}

func (c *shardChain) ShardID() int32 {
	return int32(c.shardID)
}

func (c *shardChain) GenesisBlock() interface{} {
	return &wire.MsgBlock{
		Header: chain.NewShardBlockHeader(
			chain.NewBVersion(c.chainParams.GenesisBlock.Version),
			c.chainParams.GenesisBlock.PrevBlock,
			c.chainParams.GenesisBlock.MerkleRoot,
			chainhash.Hash{},
			c.chainParams.GenesisBlock.Timestamp,
			c.chainParams.GenesisBlock.Bits,
			c.chainParams.GenesisBlock.Nonce),
		Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
	}
}

func (c *shardChain) Read(r io.Reader) (chain.BlockHeader, error) {
	h := &chain.ShardHeader{}
	err := chain.ReadShardBlockHeader(r, h)
	return h, err
}

func (c *shardChain) Write(w io.Writer, h chain.BlockHeader) error {
	header := h.(*chain.ShardHeader)
	return chain.WriteShardBlockHeader(w, header)
}

func (c *shardChain) MaxBlockHeaderPayload() int {
	return chain.MaxShardBlockHeaderPayload
}

func (c *shardChain) BlockHeaderOverhead() int {
	return chain.MaxShardBlockHeaderPayload + encoder.MaxVarIntPayload
}
