package shard

import (
	"errors"
	"time"

	"gitlab.com/jaxnet/core/shard.core/node/mining"
	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	chainParams *chaincfg.Params

	blockGenerator func(useCoinbaseValue bool) (mining.BlockTemplate, error)
}

func Chain(shardID uint32, params *chaincfg.Params, beaconGenesis *wire.BeaconHeader) *shardChain {
	shard := &shardChain{
		ShardHeaderConstructor: wire.ShardHeaderConstructor{
			ID: shardID,
		},
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
	hash := genesis.BlockHash()
	clone.GenesisHash = &hash

	return shard
}

func (c *shardChain) NewBlockHeader(ver wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits uint32, nonce uint32) (wire.BlockHeader, error) {
	header := wire.NewEmptyBeaconHeader()
	header.SetVersion(ver)
	header.SetTimestamp(timestamp)
	header.SetBits(bits)
	header.SetNonce(nonce)

	return wire.NewShardBlockHeader(
		prevHash,
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
		header := wire.NewEmptyBeaconHeader()
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

	beaconHeader, ok := blockTemplate.Block.Header.(*wire.BeaconHeader)
	if !ok {
		return nil, errors.New("invalid header type")
	}
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
