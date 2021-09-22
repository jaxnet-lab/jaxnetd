// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chain

import (
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type IChainCtx interface {
	wire.HeaderConstructor

	Name() string
	Params() *chaincfg.Params
	GenesisBlock() *wire.MsgBlock
	EmptyBlock() wire.MsgBlock
	NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode
}

var BeaconChain IChainCtx
