package shard

import (
	"math/big"
	"time"

	merged_mining_tree "gitlab.com/jaxnet/core/merged-mining-tree"
	"gitlab.com/jaxnet/core/shard.core/node/mining"
	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
	"gitlab.com/jaxnet/core/shard.core/utils/mmr"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	chainParams *chaincfg.Params
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
	h := genesis.Header.(*wire.ShardHeader)
	hash := h.BlockHash()
	clone.GenesisHash = &hash

	return shard
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

type BeaconBlockProvider struct {
	BlockGenerator func(useCoinbaseValue bool) (mining.BlockTemplate, error)
	ShardCount     func() (uint32, error)
}

type BlockGenerator struct {
	beacon BeaconBlockProvider
	mmr    mmr.IMountainRange
}

func NewChainBlockGenerator(beacon BeaconBlockProvider, mmr mmr.IMountainRange) *BlockGenerator {
	return &BlockGenerator{
		beacon: beacon,
		mmr:    mmr,
	}
}

func (c *BlockGenerator) ValidateBlock(blockHeader wire.BlockHeader) error {
	hashes, coding, codingBitSize := blockHeader.BeaconHeader().MergedMiningTreeCodingProof()
	shardHeader := blockHeader.(*wire.ShardHeader)

	count, err := c.beacon.ShardCount()
	if err != nil {
		// an error will occur if it is impossible
		// to get the last block from the chain state
		return err
	}

	tree := merged_mining_tree.NewSparseMerkleTree(count)
	rootHash := shardHeader.MergeMiningRoot()
	var validationRoot merged_mining_tree.BinHash
	copy(validationRoot[:], rootHash[:])
	return tree.Validate(codingBitSize, coding, hashes, shardHeader.MergeMiningNumber(), validationRoot)
}

func (c *BlockGenerator) AcceptBlock(blockHeader wire.BlockHeader) error {
	h := blockHeader.BlockHash()
	c.mmr.Append(big.NewInt(0), h.CloneBytes())
	return nil
}

func (c *BlockGenerator) NewBlockHeader(ver wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
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

func (c *BlockGenerator) generateBeaconHeader(ver wire.BVersion,
	timestamp time.Time, bits uint32, nonce uint32) (*wire.BeaconHeader, error) {
	if c.beacon.BlockGenerator == nil {
		header := wire.EmptyBeaconHeader()
		header.SetVersion(ver)
		header.SetTimestamp(timestamp)
		header.SetBits(bits)
		header.SetNonce(nonce)
		return header, nil
	}

	blockTemplate, err := c.beacon.BlockGenerator(true)
	if err != nil {
		return nil, err
	}

	beaconHeader := blockTemplate.Block.Header.BeaconHeader()
	beaconHeader.SetNonce(nonce)

	return beaconHeader, nil
}
