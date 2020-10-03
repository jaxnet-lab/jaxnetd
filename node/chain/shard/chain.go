package shard

import (
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core/node/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	beaconHeight int32
	startVersion wire.BVersion
	chainParams  *chaincfg.Params
}

func Chain(shardID uint32, params *chaincfg.Params, beaconGenesisBlc *wire.MsgBlock, gHeight int32) *shardChain {
	shard := &shardChain{
		ShardHeaderConstructor: wire.ShardHeaderConstructor{
			ID: shardID,
		},
		beaconHeight: gHeight,
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

	genesis := shard.GenesisBlock()
	hash := genesis.BlockHash()
	clone.GenesisHash = &hash

	return shard
}

func (c *shardChain) NewBlockHeader(version wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
	mmr chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) wire.BlockHeader {
	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return wire.NewShardBlockHeader(
		version,
		prevHash,
		merkleRootHash,
		mmr,
		timestamp,
		bits,
		nonce,
	)
}

func (c *shardChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewShardBlockNode(blockHeader, parent)
}

func (c *shardChain) Params() *chaincfg.Params {
	return c.chainParams
}

func (c *shardChain) GenesisBlock() *wire.MsgBlock {
	return &wire.MsgBlock{
		Header: wire.NewShardBlockHeader(
			wire.NewBVersion(c.chainParams.GenesisBlock.Version),
			c.chainParams.GenesisBlock.PrevBlock,
			c.chainParams.GenesisBlock.MerkleRoot,
			chainhash.Hash{},
			c.chainParams.GenesisBlock.Timestamp,
			c.chainParams.GenesisBlock.Bits,
			c.chainParams.GenesisBlock.Nonce),
		Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
	}
}

func (c *shardChain) Read(r io.Reader) (wire.BlockHeader, error) {
	h := &wire.ShardHeader{}
	err := wire.ReadShardBlockHeader(r, h)
	return h, err
}

func (c *shardChain) Write(w io.Writer, h wire.BlockHeader) error {
	header := h.(*wire.ShardHeader)
	return wire.WriteShardBlockHeader(w, header)
}
