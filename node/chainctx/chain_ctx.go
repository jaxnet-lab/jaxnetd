// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chainctx

import (
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type IChainCtx interface {
	wire.HeaderConstructor

	Name() string
	Params() *chaincfg.Params
	GenesisBlock() *wire.MsgBlock
	EmptyBlock() wire.MsgBlock

	// NewNode... todo: think about how to refactor this
	NewNode(blockHeader wire.BlockHeader, parent blocknodes.IBlockNode) blocknodes.IBlockNode
}

var BeaconChain IChainCtx = NewBeaconChain(&chaincfg.MainNetParams)
