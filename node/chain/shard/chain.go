package shard

import (
	"math/big"
	"time"

	"gitlab.com/jaxnet/core/shard.core/utils/mmr"

	"gitlab.com/jaxnet/core/shard.core/node/mining"
	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	chainParams *chaincfg.Params
	mmr         mmr.IMountainRange

	blockGenerator func(useCoinbaseValue bool) (mining.BlockTemplate, error)
}

func Chain(shardID uint32, mmr mmr.IMountainRange, params *chaincfg.Params, beaconGenesis *wire.BeaconHeader) *shardChain {
	shard := &shardChain{
		ShardHeaderConstructor: wire.ShardHeaderConstructor{
			ID: shardID,
		},
		mmr: mmr,
	}

	clone := params.ShardGenesis(shardID, nil)
	clone.GenesisBlock = chaincfg.GenesisBlockOpts{
		Version:    int32(beaconGenesis.Version()),
		Timestamp:  beaconGenesis.Timestamp(),
		PrevBlock:  chainhash.Hash{},
		MerkleRoot: chainhash.Hash{},
		Bits:       beaconGenesis.Bits(),
		Nonce:      beaconGenesis.Nonce(),
		BCHeader:   beaconGenesis,
	}
	shard.chainParams = clone

	genesis := shard.GenesisBlock()
	h := genesis.Header.(*wire.ShardHeader)
	hash := h.BlockHash()
	clone.GenesisHash = &hash

	return shard
}

func (c *shardChain) NewBlockHeader(ver wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits uint32, nonce uint32) (wire.BlockHeader, error) {
	header := wire.EmptyBeaconHeader()
	header.SetVersion(ver)
	header.SetTimestamp(timestamp)
	header.SetBits(bits)
	header.SetNonce(nonce)

	mmrRoot := c.mmr.Root()
	prevCommitment := chainhash.Hash{}
	copy(prevCommitment[:], mmrRoot[:])

	return wire.NewShardBlockHeader(
		prevCommitment,
		merkleRootHash,
		timestamp,
		bits,
		*header,
	), nil
}

func (c *shardChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewShardBlockNode(blockHeader, parent)
}

func (c *shardChain) Params() *chaincfg.Params {
	return c.chainParams
}

func (c *shardChain) EmptyBlock() wire.MsgBlock {
	return wire.EmptyShardBlock()
}

func (c *shardChain) AcceptBlock(blockHeader wire.BlockHeader) error {
	h := blockHeader.BlockHash()
	c.mmr.Append(big.NewInt(0), h.CloneBytes())
	return nil
}

func (c *shardChain) GenesisBlock() *wire.MsgBlock {
	return &wire.MsgBlock{
		Header: wire.NewShardBlockHeader(
			c.chainParams.GenesisBlock.PrevBlock,
			c.chainParams.GenesisBlock.MerkleRoot,
			c.chainParams.GenesisBlock.Timestamp,
			c.chainParams.GenesisBlock.Bits,
			*c.chainParams.GenesisBlock.BCHeader,
		),
		Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
	}
}

type HeaderGenerator struct {
	BlockGenerator func(useCoinbaseValue bool) (mining.BlockTemplate, error)
}

func (c *HeaderGenerator) generateBeaconHeader(ver wire.BVersion,
	timestamp time.Time, bits uint32, nonce uint32) (*wire.BeaconHeader, error) {
	if c.BlockGenerator == nil {
		header := wire.EmptyBeaconHeader()
		header.SetVersion(ver)
		header.SetTimestamp(timestamp)
		header.SetBits(bits)
		header.SetNonce(nonce)
		return header, nil
	}

	blockTemplate, err := c.BlockGenerator(true)
	if err != nil {
		return nil, err
	}

	beaconHeader := blockTemplate.Block.Header.BeaconHeader()
	beaconHeader.SetNonce(nonce)

	return beaconHeader, nil
}

func (c *HeaderGenerator) NewBlockHeader(ver wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits uint32, nonce uint32) (wire.BlockHeader, error) {
	header, err := c.generateBeaconHeader(ver, timestamp, bits, nonce)
	if err != nil {
		return nil, err
	}

	return wire.NewShardBlockHeader(
		prevHash,
		merkleRootHash,
		timestamp,
		bits,
		*header,
	), nil
}
