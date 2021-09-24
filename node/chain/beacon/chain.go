// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package beacon

import (
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type beaconChain struct {
	wire.BeaconHeaderConstructor
	chainParams *chaincfg.Params
}

func Chain(params *chaincfg.Params) *beaconChain {
	clone := *params
	clone.Name = "beacon"
	beacon := &beaconChain{}
	beacon.chainParams = &clone

	gb := beacon.GenesisBlock()
	hash := gb.BlockHash()
	clone.GenesisHash = &hash

	return beacon
}

func (c *beaconChain) GenesisBlock() *wire.MsgBlock {
	return &wire.MsgBlock{
		Header: wire.NewBeaconBlockHeader(
			wire.NewBVersion(c.chainParams.GenesisBlock.Version),
			c.chainParams.GenesisBlock.PrevBlock, // todo: put actual MMR Root
			c.chainParams.GenesisBlock.MerkleRoot,
			chainhash.Hash{},
			c.chainParams.GenesisBlock.Timestamp,
			c.chainParams.GenesisBlock.Bits,
			c.chainParams.GenesisBlock.Nonce,
		),
		Transactions: []*wire.MsgTx{&chaincfg.GenesisCoinbaseTx},
	}
}

func (c *beaconChain) Params() *chaincfg.Params {
	return c.chainParams
}

func (c *beaconChain) Name() string {
	return c.chainParams.Name
}

func (c *beaconChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewBeaconBlockNode(blockHeader, parent)
}

func (c *beaconChain) EmptyBlock() wire.MsgBlock {
	return wire.EmptyBeaconBlock()
}
