package beacon

import (
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
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
	chainParams *chaincfg.Params
}

func Chain(params *chaincfg.Params) chain.IChain {
	clone := *params
	clone.Name = params.Name + "_beacon"
	return &beaconChain{chainParams: &clone}
}

func (c *beaconChain) GenesisBlock() interface{} {
	return &wire.MsgBlock{
		Header:       NewBlockHeader(1, chainhash.Hash{}, genesisMerkleRoot, chainhash.Hash{}, time.Unix(0x495fab29, 0), 0x1d00ffff, 0x7c2bac1d),
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
	return &header{}
}

func (c *beaconChain) NewBlockHeader(version int32, prevHash, merkleRootHash chainhash.Hash,
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
		timestamp:       timestamp, // time.Unix(time.Now().Unix(), 0),
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
