package chain

import (
	"time"

	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type IChainCtx interface {
	wire.HeaderConstructor

	Params() *chaincfg.Params
	GenesisBlock() *wire.MsgBlock
	EmptyBlock() wire.MsgBlock

	NewBlockHeader(version wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
		timestamp time.Time, bits uint32, nonce uint32) (wire.BlockHeader, error)
	NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode

	AcceptBlock(blockHeader wire.BlockHeader) error
}

var BeaconChain IChainCtx
