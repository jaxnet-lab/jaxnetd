package beacon

import (
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
	maxBlockHeaderPayload = 16 + (chainhash.HashSize * 3)
)

type beaconChain struct {
	chainParams *chaincore.Params
}

func Chain(params *chaincore.Params) chain.IChain {
	clone := *params
	clone.Name = "beacon"
	beacon := &beaconChain{}
	beacon.chainParams = &clone

	gb := beacon.GenesisBlock().(*wire.MsgBlock)
	hash := gb.BlockHash()
	clone.GenesisHash = &hash

	return beacon
}

func (c *beaconChain) GenesisBlock() interface{} {
	return &wire.MsgBlock{
		Header: NewBlockHeader(
			chain.NewBVersion(c.chainParams.GenesisBlock.Version),
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

func (c *beaconChain) IsBeacon() bool {
	return true
}

func (c *beaconChain) Params() *chaincore.Params {
	return c.chainParams
}

func (c *beaconChain) ShardID() int32 {
	return 0
}

func (c *beaconChain) NewHeader() chain.BlockHeader {
	return &header{}
}

func (c *beaconChain) NewBlockHeader(version chain.BVersion, prevHash, merkleRootHash chainhash.Hash,
	mergeMiningRoot chainhash.Hash,
	timestamp time.Time,
	bits uint32, nonce uint32) chain.BlockHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &header{
		version:         version,
		prevBlock:       prevHash,
		merkleRoot:      merkleRootHash,
		mergeMiningRoot: mergeMiningRoot,
		timestamp:       timestamp,
		bits:            bits,
		nonce:           nonce,
	}
}

func (c *beaconChain) NewNode(blockHeader chain.BlockHeader, parent chain.IBlockNode) chain.IBlockNode {
	return BlockNode(blockHeader, parent)
}

func (c *beaconChain) MaxBlockHeaderPayload() int {
	return maxBlockHeaderPayload
}

func (c *beaconChain) BlockHeaderOverhead() int {
	return maxBlockHeaderPayload + encoder.MaxVarIntPayload
}
