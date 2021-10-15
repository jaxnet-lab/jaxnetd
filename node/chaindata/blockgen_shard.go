/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	mmtree "gitlab.com/jaxnet/jaxnetd/types/merge_mining_tree"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type ChainBlockGenerator interface {
	NewBlockHeader(version wire.BVersion, blocksMMRRoot, merkleRootHash chainhash.Hash,
		timestamp time.Time, bits, nonce uint32, burnReward int) (wire.BlockHeader, error)

	ValidateJaxAuxRules(block *wire.MsgBlock, height int32) error

	CalcBlockSubsidy(height int32, header wire.BlockHeader) int64
}

type BeaconBlockProvider interface {
	BlockTemplate(useCoinbaseValue bool, burnReward int) (BlockTemplate, error)
	ShardCount() (uint32, error)
	BestSnapshot() *BestState
	CalcKForHeight(height int32) uint32
}

type ShardBlockGenerator struct {
	beacon BeaconBlockProvider
	ctx    chainctx.IChainCtx
}

func NewShardBlockGen(ctx chainctx.IChainCtx, beacon BeaconBlockProvider) *ShardBlockGenerator {
	return &ShardBlockGenerator{beacon: beacon, ctx: ctx}
}

func (c *ShardBlockGenerator) NewBlockHeader(_ wire.BVersion, blocksMMRRoot, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits, nonce uint32, burnReward int) (wire.BlockHeader, error) {
	header, cAux, err := c.generateBeaconHeader(nonce, timestamp, burnReward)
	if err != nil {
		return nil, err
	}

	return wire.NewShardBlockHeader(blocksMMRRoot, merkleRootHash, bits, *header, cAux), nil
}

func (c *ShardBlockGenerator) ValidateMergeMiningData(header wire.BlockHeader) error {
	actualShardsCount := c.beacon.BestSnapshot().Shards

	beaconAux := header.BeaconHeader()
	if math.Abs(float64(actualShardsCount-beaconAux.Shards())) > 1 {
		return fmt.Errorf("delta between actualShardsCount(%v) and beaconAux.Shards(%v) more than 1",
			actualShardsCount, beaconAux.Shards())
	}

	mmNumber := header.MergeMiningNumber()
	if mmNumber > beaconAux.Shards() {
		return fmt.Errorf("MergeMiningNumber(%v) more than beaconAux.Shards(%v)",
			mmNumber, beaconAux.Shards())
	}

	tree := mmtree.NewSparseMerkleTree(beaconAux.Shards())

	orangeTreeEmpty := chainhash.NextPowerOfTwo(int(mmNumber)) == int(mmNumber) && mmNumber == beaconAux.Shards()
	if !orangeTreeEmpty {
		hashes, coding, codingBitsLen := beaconAux.MergedMiningTreeCodingProof()
		err := tree.ValidateOrangeTree(codingBitsLen, coding, hashes, mmNumber, beaconAux.MergeMiningRoot())
		if err != nil {
			return errors.Wrap(err, "invalid orange tree")
		}
	}

	position := c.ctx.ShardID() - 1
	exclusiveHash := header.ExclusiveHash()
	err := tree.ValidateShardMerkleProofPath(position, beaconAux.Shards(), header.ShardMerkleProof(),
		exclusiveHash, beaconAux.MergeMiningRoot())
	if err != nil {
		return errors.Wrap(err, "invalid shard merkle proof")
	}
	return nil
}

func (c *ShardBlockGenerator) ValidateJaxAuxRules(block *wire.MsgBlock, height int32) error {
	err := c.ValidateMergeMiningData(block.Header)
	if err != nil {
		return err
	}

	expectedReward := c.CalcBlockSubsidy(height, block.Header)

	shardHeader := block.Header.(*wire.ShardHeader)
	shardCoinbaseTx := block.Transactions[0]

	return ValidateShardCoinbase(shardHeader, shardCoinbaseTx, expectedReward)
}

func (c *ShardBlockGenerator) CalcBlockSubsidy(height int32, header wire.BlockHeader) int64 {
	relativeBeaconHeight := c.ctx.GenesisBeaconHeight() + (height / 16)
	kVal := c.beacon.CalcKForHeight(relativeBeaconHeight)

	reward := CalcShardBlockSubsidy(header.MergeMiningNumber(), header.Bits(), kVal)

	if c.ctx.Params().Net != types.MainNet && reward < chaincfg.ShardTestnetBaseReward*chaincfg.JuroPerJAXCoin {
		return chaincfg.ShardTestnetBaseReward * chaincfg.JuroPerJAXCoin
	}

	return reward
}

func (c *ShardBlockGenerator) generateBeaconHeader(nonce uint32, timestamp time.Time, burnReward int) (*wire.BeaconHeader, wire.CoinbaseAux, error) {
	blockTemplate, err := c.beacon.BlockTemplate(false, burnReward)
	if err != nil {
		return nil, wire.CoinbaseAux{}, err
	}

	coinbaseAux := wire.CoinbaseAux{
		Tx:            *blockTemplate.Block.Transactions[0].Copy(),
		TxMerkleProof: make([]chainhash.Hash, len(blockTemplate.Block.Transactions)),
	}

	for i, tx := range blockTemplate.Block.Transactions {
		coinbaseAux.TxMerkleProof[i] = tx.TxHash()
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

	dRat := new(big.Float).SetInt(d)
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
