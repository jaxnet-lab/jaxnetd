/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package shard

import (
	"fmt"
	"math/big"
	"time"

	mmtree "gitlab.com/jaxnet/core/merged-mining-tree"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type BeaconBlockProvider interface {
	BlockTemplate(useCoinbaseValue bool, burnReward int) (mining.BlockTemplate, error)
	ShardCount() (uint32, error)
}

type BlockGenerator struct {
	beacon BeaconBlockProvider
}

func NewChainBlockGenerator(beacon BeaconBlockProvider) *BlockGenerator {
	return &BlockGenerator{beacon: beacon}
}

func (c *BlockGenerator) NewBlockHeader(_ wire.BVersion, blocksMMRRoot, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits, nonce uint32, burnReward int) (wire.BlockHeader, error) {
	header, cAux, err := c.generateBeaconHeader(nonce, timestamp, burnReward)
	if err != nil {
		return nil, err
	}

	return wire.NewShardBlockHeader(blocksMMRRoot, merkleRootHash, bits, *header, cAux), nil
}

func (c *BlockGenerator) ValidateBlockHeader(header wire.BlockHeader) error {
	lastKnownShardsAmount, err := c.beacon.ShardCount()
	if err != nil {
		// An error will occur if it is impossible
		// to get the last block from the chain state.
		return fmt.Errorf("can't fetch last beacon block: %w", err)
	}

	beaconHeader := header.BeaconHeader()
	shardHeader := header.(*wire.ShardHeader)

	treeValidationShouldBeSkipped := false
	mmNumber := shardHeader.MergeMiningNumber()
	if mmNumber%2 == 0 {
		if header.BeaconHeader().Shards() == mmNumber {
			if mmNumber <= lastKnownShardsAmount {
				treeValidationShouldBeSkipped = true
			}
		}
	}

	if !treeValidationShouldBeSkipped {
		hashes, coding, codingBitsLen := beaconHeader.MergedMiningTreeCodingProof()

		var (
			providedRoot   = shardHeader.MergeMiningRoot()
			validationRoot mmtree.BinHash
		)
		copy(validationRoot[:], providedRoot[:])

		tree := mmtree.NewSparseMerkleTree(lastKnownShardsAmount)
		err = tree.Validate(codingBitsLen, coding, hashes, mmNumber, validationRoot)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *BlockGenerator) ValidateCoinbaseTx(block *wire.MsgBlock, height int32, net types.JaxNet) error {
	expectedReward := c.CalcBlockSubsidy(height, block.Header, net)

	shardHeader := block.Header.(*wire.ShardHeader)
	shardCoinbaseTx := block.Transactions[0]

	return mining.ValidateShardCoinbase(shardHeader, shardCoinbaseTx, expectedReward)
}

func (c *BlockGenerator) AcceptBlock(wire.BlockHeader) error {
	return nil
}

func (c *BlockGenerator) CalcBlockSubsidy(_ int32, header wire.BlockHeader, net types.JaxNet) int64 {
	reward := CalcShardBlockSubsidy(header.(*wire.ShardHeader).MergeMiningNumber(), header.Bits(), header.K())

	if net != types.MainNet && reward < chaincfg.ShardTestnetBaseReward*chaincfg.JuroPerJAXCoin {
		return chaincfg.ShardTestnetBaseReward * chaincfg.JuroPerJAXCoin
	}

	return reward
}

func (c *BlockGenerator) generateBeaconHeader(nonce uint32, timestamp time.Time, burnReward int) (*wire.BeaconHeader, wire.CoinbaseAux, error) {
	blockTemplate, err := c.beacon.BlockTemplate(false, burnReward)
	if err != nil {
		return nil, wire.CoinbaseAux{}, err
	}

	coinbaseAux := wire.CoinbaseAux{
		Tx:       *blockTemplate.Block.Transactions[0].Copy(),
		TxMerkle: make([]chainhash.Hash, len(blockTemplate.Block.Transactions)),
	}

	for i, tx := range blockTemplate.Block.Transactions {
		coinbaseAux.TxMerkle[i] = tx.TxHash()
	}

	beaconHeader := blockTemplate.Block.Header.BeaconHeader()
	beaconHeader.SetNonce(nonce)
	beaconHeader.SetTimestamp(timestamp)

	return beaconHeader, coinbaseAux, nil
}

// CalcShardBlockSubsidy returns reward for shard block.
// - height is block height;
// - shards is a number of shards that were mined by a miner at the time;
// - bits is current target;
// - k is inflation-fix-coefficient.
func CalcShardBlockSubsidy(shards, bits, k uint32) int64 {
	// ((Di * Ki) / n)  * jaxutil.SatoshiPerJAXCoin
	d := pow.CalcWork(bits)
	k1 := pow.UnpackK(k)

	if shards == 0 {
		shards = 1
	}

	dRat := new(big.Float).SetInt64(d.Int64())
	shardsN := new(big.Float).SetInt64(int64(shards))

	// (Di * Ki)
	dk := new(big.Float).Mul(dRat, k1)
	// ((Di * Ki) / n)
	reward, _ := new(big.Float).Quo(dk, shardsN).Float64()
	if reward == 0 {
		return 0
	}

	return int64(reward * 1_0000)
}
