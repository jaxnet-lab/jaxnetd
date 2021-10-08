// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package beacon

import (
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type beaconChain struct {
	wire.BeaconHeaderConstructor
	chainParams *chaincfg.Params
}

func Chain(params *chaincfg.Params) *beaconChain {
	clone := *params
	clone.Name = "beacon"
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

func (c *beaconChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewBeaconBlockNode(blockHeader, parent, c.chainParams.PowParams.PowLimitBits)
}
