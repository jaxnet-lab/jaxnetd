package chain

import (
	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type IChainCtx interface {
	wire.HeaderConstructor

	Params() *chaincfg.Params
	GenesisBlock() *wire.MsgBlock
	EmptyBlock() wire.MsgBlock
	NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode
}

var BeaconChain IChainCtx
