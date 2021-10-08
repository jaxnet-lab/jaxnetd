// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package shard

import (
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	chainParams *chaincfg.Params
}

func Chain(shardID uint32, params *chaincfg.Params, beaconBlock *wire.MsgBlock) *shardChain {
	chainParams := params.ShardParams(shardID, beaconBlock)

	shard := &shardChain{
		ShardHeaderConstructor: wire.ShardHeaderConstructor{ID: shardID},
		chainParams:            chainParams,
	}

	return shard
}

func (c *shardChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewShardBlockNode(blockHeader, parent, c.chainParams.PowParams.PowLimitBits)
}

func (c *shardChain) Params() *chaincfg.Params     { return c.chainParams }
func (c *shardChain) Name() string                 { return c.chainParams.ChainName }
func (c *shardChain) EmptyBlock() wire.MsgBlock    { return wire.EmptyShardBlock() }
func (c *shardChain) GenesisBlock() *wire.MsgBlock { return c.chainParams.GenesisBlock() }
