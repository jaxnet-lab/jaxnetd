/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chainctx

import (
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type beaconChain struct {
	wire.BeaconHeaderConstructor
	chainParams *chaincfg.Params
}

func NewBeaconChain(params *chaincfg.Params) *beaconChain {
	clone := *params
	clone.ChainName = "beacon"
	clone.IsBeacon = true
	beacon := &beaconChain{
		chainParams: &clone,
	}
	return beacon
}

func (c *beaconChain) Name() string                 { return c.chainParams.ChainName }
func (c *beaconChain) Params() *chaincfg.Params     { return c.chainParams }
func (c *beaconChain) EmptyBlock() wire.MsgBlock    { return wire.EmptyBeaconBlock() }
func (c *beaconChain) GenesisBlock() *wire.MsgBlock { return c.chainParams.GenesisBlock() }
func (c *beaconChain) GenesisBeaconHeight() int32   { return 0 }

func (c *beaconChain) NewNode(blockHeader wire.BlockHeader, parent blocknodes.IBlockNode, serialID int64) blocknodes.IBlockNode {
	return blocknodes.NewBeaconBlockNode(blockHeader, parent, serialID)
}
