/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chainctx

import (
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	chainParams  *chaincfg.Params
	beaconHeight int32
}

func ShardChain(shardID uint32, params *chaincfg.Params, beaconBlock *wire.MsgBlock, height int32) *shardChain {
	chainParams := params.ShardParams(shardID, beaconBlock)

	shard := &shardChain{
		ShardHeaderConstructor: wire.ShardHeaderConstructor{ID: shardID},
		chainParams:            chainParams,
		beaconHeight:           height,
	}

	return shard
}

func (c *shardChain) NewNode(blockHeader wire.BlockHeader, parent blocknodes.IBlockNode, serialID int64) blocknodes.IBlockNode {
	powWeight := pow.CalcPowWeight(
		pow.CompactToBig(c.chainParams.PowParams.PowLimitBits),
		blockHeader.Bits(),
		c.chainParams.PowParams.HashSortingSlotNumber,
	)
	return blocknodes.NewShardBlockNode(blockHeader, parent, serialID, powWeight)
}

func (c *shardChain) Params() *chaincfg.Params     { return c.chainParams }
func (c *shardChain) Name() string                 { return c.chainParams.ChainName }
func (c *shardChain) GenesisBlock() *wire.MsgBlock { return c.chainParams.GenesisBlock() }
func (c *shardChain) GenesisBeaconHeight() int32   { return c.beaconHeight }
