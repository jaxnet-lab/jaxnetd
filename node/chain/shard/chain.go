// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package shard

import (
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type shardChain struct {
	wire.ShardHeaderConstructor
	chainParams *chaincfg.Params
}

func Chain(shardID uint32, params *chaincfg.Params, beaconGenesis *wire.BeaconHeader) *shardChain {
	shard := &shardChain{
		ShardHeaderConstructor: wire.ShardHeaderConstructor{ID: shardID},
	}

	clone := params.ShardGenesis(shardID, nil)
	clone.GenesisBlock = chaincfg.GenesisBlockOpts{
		Version:    int32(beaconGenesis.Version()),
		Timestamp:  beaconGenesis.Timestamp(),
		PrevBlock:  chainhash.Hash{},
		MerkleRoot: chainhash.Hash{},
		Bits:       beaconGenesis.Bits(),
		Nonce:      beaconGenesis.Nonce(),
		BCHeader:   *beaconGenesis,
	}
	shard.chainParams = clone

	genesis := shard.GenesisBlock()
	h := genesis.Header.(*wire.ShardHeader)
	hash := h.BlockHash()
	clone.GenesisHash = &hash

	return shard
}

func (c *shardChain) NewNode(blockHeader wire.BlockHeader, parent blocknode.IBlockNode) blocknode.IBlockNode {
	return blocknode.NewShardBlockNode(blockHeader, parent)
}

func (c *shardChain) Params() *chaincfg.Params {
	return c.chainParams
}
func (c *shardChain) Name() string {
	return c.chainParams.Name
}

func (c *shardChain) EmptyBlock() wire.MsgBlock {
	return wire.EmptyShardBlock()
}

func (c *shardChain) GenesisBlock() *wire.MsgBlock {
	return &wire.MsgBlock{
		ShardBlock: true,
		Header: wire.NewShardBlockHeader(
			c.chainParams.GenesisBlock.PrevBlock,
			c.chainParams.GenesisBlock.MerkleRoot,
			c.chainParams.GenesisBlock.Timestamp,
			c.chainParams.GenesisBlock.Bits,
			c.chainParams.GenesisBlock.BCHeader,
			wire.CoinbaseAux{}.New(), // todo: put the real coinbase tx from beacon
		),
		Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
	}
}
