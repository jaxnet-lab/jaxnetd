package beacon

import (
	"time"

	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type beaconChain struct {
	wire.BeaconHeaderConstructor
	chainParams *chaincfg.Params
}

func Chain(params *chaincfg.Params) *beaconChain {
	clone := *params
	clone.Name = "beacon"
	beacon := &beaconChain{}
	beacon.chainParams = &clone

	gb := beacon.GenesisBlock()
	hash := gb.BlockHash()
	clone.GenesisHash = &hash

	return beacon
}

func (c *beaconChain) GenesisBlock() *wire.MsgBlock {
	return &wire.MsgBlock{
		Header: wire.NewBeaconBlockHeader(
			wire.NewBVersion(c.chainParams.GenesisBlock.Version),
			c.chainParams.GenesisBlock.PrevBlock,
			c.chainParams.GenesisBlock.MerkleRoot,
			chainhash.Hash{},
			c.chainParams.GenesisBlock.Timestamp,
			c.chainParams.GenesisBlock.Bits,
			c.chainParams.GenesisBlock.Nonce,
		),
		Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
	}
}

func (c *beaconChain) Params() *chaincfg.Params {
	return c.chainParams
}

func (c *beaconChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewBeaconBlockNode(blockHeader, parent)
}

func (c *beaconChain) EmptyBlock() wire.MsgBlock {
	return wire.EmptyBeaconBlock()
}

type StateProvider struct {
	ShardCount func() (uint32, error)
}

type BlockGenerator struct {
	stateInfo StateProvider
}

func NewChainBlockGenerator(stateInfo StateProvider) *BlockGenerator {
	return &BlockGenerator{
		stateInfo: stateInfo,
	}
}

func (c *BlockGenerator) NewBlockHeader(version wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits uint32, nonce uint32) (wire.BlockHeader, error) {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	header := wire.NewBeaconBlockHeader(
		version,
		prevHash,
		merkleRootHash,
		chainhash.Hash{},
		timestamp,
		bits,
		nonce,
	)

	count, err := c.stateInfo.ShardCount()
	if err != nil {
		// an error will occur if it is impossible
		// to get the last block from the chain state
		return header, err
	}

	header.SetShards(count)

	if version.ExpansionMade() {
		header.SetShards(count + 1)
	}

	return header, nil
}

func (c *BlockGenerator) ValidateBlock(wire.BlockHeader) error {
	return nil
}

func (c *BlockGenerator) AcceptBlock(wire.BlockHeader) error {
	return nil
}
