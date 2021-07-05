// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package shard

import (
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	chainParams chaincfg.Params
	genesisTx   wire.MsgTx
}

func Chain(shardID uint32, params *chaincfg.Params, beaconGenesis *wire.BeaconHeader, tx *wire.MsgTx) *shardChain {
	shard := &shardChain{
		ShardHeaderConstructor: wire.ShardHeaderConstructor{ID: shardID},
		genesisTx:              *tx.Copy(),
	}

	chainParams := params.ShardGenesis(shardID, nil)
	chainParams.GenesisBlock = chaincfg.GenesisBlockOpts{
		Version:    int32(beaconGenesis.Version()),
		Timestamp:  beaconGenesis.Timestamp(),
		PrevBlock:  chainhash.Hash{},
		MerkleRoot: chainhash.Hash{},
		Bits:       pow.ShardGenesisDifficulty(beaconGenesis.Bits()),
		Nonce:      beaconGenesis.Nonce(),
		BCHeader:   *beaconGenesis,
	}

	chainParams.PowLimitBits = pow.ShardGenesisDifficulty(beaconGenesis.Bits())
	if params.Net == chaincfg.FastNetParams.Net {
		chainParams.PowLimitBits = chaincfg.FastNetParams.PowLimitBits
	}

	shard.SetChainParams(*chainParams)
	return shard
}

func (c *shardChain) SetChainParams(params chaincfg.Params) {
	c.chainParams = params

	genesis := c.GenesisBlock().BlockHash()
	c.chainParams.GenesisHash = &genesis
}

func (c *shardChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewShardBlockNode(blockHeader, parent)
}

func (c *shardChain) Params() *chaincfg.Params  { return &c.chainParams }
func (c *shardChain) Name() string              { return c.chainParams.Name }
func (c *shardChain) EmptyBlock() wire.MsgBlock { return wire.EmptyShardBlock() }

func (c *shardChain) GenesisBlock() *wire.MsgBlock {
	return &wire.MsgBlock{
		ShardBlock: true,
		Header: wire.NewShardBlockHeader(
			c.chainParams.GenesisBlock.PrevBlock,
			c.chainParams.GenesisBlock.MerkleRoot,
			c.chainParams.GenesisBlock.Timestamp,
			c.chainParams.GenesisBlock.Bits,
			c.chainParams.GenesisBlock.BCHeader,
			wire.CoinbaseAux{
				Tx:       *c.genesisTx.Copy(),
				TxMerkle: []chainhash.Hash{c.chainParams.GenesisBlock.MerkleRoot},
			},
		),
		Transactions: []*wire.MsgTx{&c.genesisTx},
	}
}
