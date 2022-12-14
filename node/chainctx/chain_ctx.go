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
	GenesisBeaconHeight() int32

	NewNode(blockHeader wire.BlockHeader, parent blocknodes.IBlockNode, serialID int64) blocknodes.IBlockNode
}

var BeaconChain IChainCtx = NewBeaconChain(&chaincfg.MainNetParams)
