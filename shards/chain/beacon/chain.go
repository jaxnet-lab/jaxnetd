package beacon

import (
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
)

type beaconChain struct {
	chainParams *chaincfg.Params
}

func Chain(params *chaincfg.Params) chain.IChain {
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
		Header: chain.NewBeaconBlockHeader(
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

func (c *beaconChain) Params() *chaincfg.Params {
	return c.chainParams
}

func (c *beaconChain) ShardID() int32 {
	return 0
}

func (c *beaconChain) NewHeader() chain.BlockHeader {
	return &chain.BeaconHeader{}
}

func (c *beaconChain) NewBlockHeader(version chain.BVersion, prevHash, merkleRootHash chainhash.Hash,
	mergeMiningRoot chainhash.Hash,
	timestamp time.Time,
	bits uint32, nonce uint32) chain.BlockHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return chain.NewBeaconBlockHeader(
		version,
		prevHash,
		merkleRootHash,
		mergeMiningRoot,
		timestamp,
		bits,
		nonce,
	)
}

func (c *beaconChain) NewNode(blockHeader chain.BlockHeader, parent chain.IBlockNode) chain.IBlockNode {
	return chain.NewBeaconBlockNode(blockHeader, parent)
}

func (c *beaconChain) MaxBlockHeaderPayload() int {
	return chain.MaxBeaconBlockHeaderPayload
}

func (c *beaconChain) BlockHeaderOverhead() int {
	return chain.MaxBeaconBlockHeaderPayload + encoder.MaxVarIntPayload
}
